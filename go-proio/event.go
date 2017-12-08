package proio // import "github.com/decibelcooper/proio/go-proio"

// Generate protobuf messages
//go:generate bash gen.sh

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"github.com/decibelcooper/proio/go-proio/proto"
	protobuf "github.com/golang/protobuf/proto"
)

type Event struct {
	Err error

	proto *proto.Event

	revTypeLookup  map[string]uint64
	revTagLookup   map[uint64][]string
	entryTypeCache map[uint64]reflect.Type
	entryCache     map[uint64]protobuf.Message
}

func NewEvent() *Event {
	return &Event{
		proto: &proto.Event{
			Entries: make(map[uint64]*proto.Entry),
			Types:   make(map[uint64]string),
			Tags:    make(map[string]*proto.Tag),
		},
		revTypeLookup:  make(map[string]uint64),
		revTagLookup:   make(map[uint64][]string),
		entryTypeCache: make(map[uint64]reflect.Type),
		entryCache:     make(map[uint64]protobuf.Message),
	}
}

func (evt *Event) AddEntry(entry protobuf.Message, tags ...string) uint64 {
	typeID := evt.getTypeID(entry)
	entryProto := &proto.Entry{
		Type: typeID,
	}

	evt.proto.NEntries++
	id := evt.proto.NEntries
	evt.proto.Entries[id] = entryProto

	evt.entryCache[id] = entry

	for _, tag := range tags {
		evt.tagEntry(id, tag)
	}

	return id
}

func (evt *Event) AddEntries(tag string, entries ...protobuf.Message) []uint64 {
	var ids []uint64
	for _, entry := range entries {
		ids = append(ids, evt.AddEntry(entry, tag))
	}
	return ids
}

func (evt *Event) GetEntry(id uint64) protobuf.Message {
	entry, ok := evt.entryCache[uint64(id)]
	if ok {
		evt.Err = nil
		return entry
	}

	entryProto, ok := evt.proto.Entries[uint64(id)]
	if !ok {
		evt.Err = errors.New("no such entry: " + strconv.FormatUint(id, 10))
		return nil
	}

	entry = evt.getPrototype(entryProto.Type)
	if entry == nil {
		evt.Err = errors.New("unknown type: " + evt.proto.Types[entryProto.Type])
		return nil
	}
	selfSerializingEntry, ok := entry.(selfSerializingEntry)
	if ok {
		if err := selfSerializingEntry.Unmarshal(entryProto.Payload); err != nil {
			evt.Err = errors.New(
				"failure to unmarshal entry " +
					strconv.FormatUint(id, 10) +
					" with type " +
					evt.proto.Types[entryProto.Type],
			)
			return nil
		}
	} else {
		if err := protobuf.Unmarshal(entryProto.Payload, entry); err != nil {
			evt.Err = errors.New(
				"failure to unmarshal entry " +
					strconv.FormatUint(id, 10) +
					" with type " +
					evt.proto.Types[entryProto.Type],
			)
			return nil
		}
	}

	evt.entryCache[id] = entry

	evt.Err = nil
	return entry
}

func (evt *Event) RemoveEntry(id uint64) {
	tags := evt.EntryTags(id)
	for _, tag := range tags {
		tagProto := evt.proto.Tags[tag]
		for i, thisID := range tagProto.Entries {
			if thisID == id {
				tagProto.Entries = append(tagProto.Entries[:i], tagProto.Entries[i+1:]...)
			}
		}
	}

	delete(evt.revTagLookup, id)
	delete(evt.entryCache, id)
	delete(evt.proto.Entries, id)
}

func (evt *Event) AllEntries() []uint64 {
	var IDs []uint64
	for ID, _ := range evt.proto.Entries {
		IDs = append(IDs, ID)
	}
	return IDs
}

func (evt *Event) TaggedEntries(tag string) []uint64 {
	tagProto, ok := evt.proto.Tags[tag]
	if ok {
		return tagProto.Entries[:]
	}
	return nil
}

func (evt *Event) Tags() []string {
	var tags []string
	for key, _ := range evt.proto.Tags {
		tags = append(tags, key)
	}
	sort.Strings(tags)
	return tags
}

func (evt *Event) EntryTags(id uint64) []string {
	tags, ok := evt.revTagLookup[id]
	if ok {
		return tags
	}

	tags = make([]string, 0)
	for name, tagProto := range evt.proto.Tags {
		for _, thisID := range tagProto.Entries {
			if thisID == id {
				tags = append(tags, name)
				break
			}
		}
	}

	evt.revTagLookup[id] = tags

	return tags
}

func (evt *Event) RemoveTag(tag string) {
	delete(evt.proto.Tags, tag)
}

func (evt *Event) String() string {
	var printString string

	tags := evt.Tags()

	for _, tag := range tags {
		printString += "Tag: " + tag + "\n"
		entries := evt.TaggedEntries(tag)
		for _, entryID := range entries {
			printString += fmt.Sprintf("ID:%v ", entryID)
			entry := evt.GetEntry(entryID)
			if entry != nil {
				printString += fmt.Sprintln(entry)
			}
		}
	}

	return printString
}

type selfSerializingEntry interface {
	protobuf.Message

	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}

func newEventFromProto(eventProto *proto.Event) *Event {
	if eventProto.Entries == nil {
		eventProto.Entries = make(map[uint64]*proto.Entry)
	}
	if eventProto.Types == nil {
		eventProto.Types = make(map[uint64]string)
	}
	if eventProto.Tags == nil {
		eventProto.Tags = make(map[string]*proto.Tag)
	}
	return &Event{
		proto:          eventProto,
		revTypeLookup:  make(map[string]uint64),
		revTagLookup:   make(map[uint64][]string),
		entryTypeCache: make(map[uint64]reflect.Type),
		entryCache:     make(map[uint64]protobuf.Message),
	}
}

func (evt *Event) getPrototype(id uint64) protobuf.Message {
	entryType, ok := evt.entryTypeCache[id]
	if !ok {
		ptrType := protobuf.MessageType(evt.proto.Types[id])
		if ptrType == nil {
			return nil
		}
		entryType = ptrType.Elem()
		evt.entryTypeCache[id] = entryType
	}

	return reflect.New(entryType).Interface().(protobuf.Message)
}

func (evt *Event) getTypeID(entry protobuf.Message) uint64 {
	typeName := protobuf.MessageName(entry)
	typeID, ok := evt.revTypeLookup[typeName]
	if !ok {
		for id, name := range evt.proto.Types {
			if name == typeName {
				evt.revTypeLookup[typeName] = id
				return id
			}
		}

		evt.proto.NTypes++
		typeID = evt.proto.NTypes
		evt.proto.Types[typeID] = typeName
		evt.revTypeLookup[typeName] = typeID
	}

	return typeID
}

func (evt *Event) tagEntry(id uint64, tag string) {
	var tagProto *proto.Tag
	for name, thisTagProto := range evt.proto.Tags {
		if name == tag {
			tagProto = thisTagProto
		}
	}

	if tagProto == nil {
		tagProto = &proto.Tag{}
		evt.proto.Tags[tag] = tagProto
	}

	tagProto.Entries = append(tagProto.Entries, id)
}

func (evt *Event) flushCache() {
	for id, entry := range evt.entryCache {
		selfSerializingEntry, ok := entry.(selfSerializingEntry)
		var bytes []byte
		if ok {
			bytes, _ = selfSerializingEntry.Marshal()
		} else {
			bytes, _ = protobuf.Marshal(entry)
		}
		evt.proto.Entries[id].Payload = bytes
	}
	evt.entryCache = make(map[uint64]protobuf.Message)
}

func fromProto(bytes []byte) *Event {
	eventProto := &proto.Event{}
	err := eventProto.Unmarshal(bytes)
	if err != nil {
		return nil
	}
	return &Event{
		proto: eventProto,
	}
}
