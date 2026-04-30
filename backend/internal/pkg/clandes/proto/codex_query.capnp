@0xa5353c64839c32f2;

using Go = import "/go.capnp";
$Go.package("proto");
$Go.import("github.com/Wei-Shaw/sub2api/internal/pkg/clandes/proto");

interface CodexQueryService {
  getProfile @0 (accountId :Text) -> (
    success          :Bool,
    message          :Text,
    accountId        :Text,
    chatgptAccountId :Text,
    email            :Text,
    planType         :Text
  );
}
