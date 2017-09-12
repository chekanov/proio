#ifndef EVENT_H
#define EVENT_H

#include <map>
#include <vector>

#include "eicio.pb.h"

namespace eicio {
class Event {
   public:
    Event();
    virtual ~Event();

    bool Add(google::protobuf::Message *coll, std::string name);
    void Remove(std::string collName);
    google::protobuf::Message *Get(std::string collName);
    std::vector<std::string> GetNames();

    void MakeReference(google::protobuf::Message *msg, eicio::Reference *ref);
    google::protobuf::Message *Dereference(const eicio::Reference &ref);

    unsigned int GetUniqueID();
    void SetHeader(EventHeader *newHeader);
    EventHeader *GetHeader();
    unsigned int GetPayloadSize();
    void *SetPayloadSize(google::protobuf::uint32 size);
    unsigned char *GetPayload();
    std::string GetType(google::protobuf::Message *coll);
    void FlushCollCache();

   private:
    google::protobuf::Message *getFromPayload(std::string collName, bool parse = true);

    void collToPayload(google::protobuf::Message *coll, std::string name);

    EventHeader *header;
    std::vector<unsigned char> payload;

    std::map<std::string, google::protobuf::Message *> collCache;
    std::vector<std::string> namesCached;
};
}

#endif  // EVENT_H