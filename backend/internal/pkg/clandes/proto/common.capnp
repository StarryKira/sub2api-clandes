@0xa0b1c2d3e4f5a6b7;

using Go = import "/go.capnp";
$Go.package("proto");
$Go.import("github.com/Wei-Shaw/sub2api/internal/pkg/clandes/proto");

struct ProxyInfo {
  ip             @0 :Text;
  country        @1 :Text;
  region         @2 :Text;
  city           @3 :Text;
  continent      @4 :Text;
  asn            @5 :UInt32;
  asOrganization @6 :Text;
  colo           @7 :Text;
  timezone       @8 :Text;
}
