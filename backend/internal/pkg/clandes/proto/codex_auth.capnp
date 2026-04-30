@0xf98c6ed908b33113;

using Go = import "/go.capnp";
$Go.package("proto");
$Go.import("github.com/Wei-Shaw/sub2api/internal/pkg/clandes/proto");

interface CodexAuthService {
  startLogin @0 (
    redirectUri :Text,
    proxyUrl    :Text
  ) -> (authUrl :Text, sessionId :Text);

  completeLogin @1 (
    sessionId :Text,
    code      :Text
  ) -> (
    success           :Bool,
    message           :Text,
    accountId         :Text,
    accessToken       :Text,
    refreshToken      :Text,
    idToken           :Text,
    expiresIn         :UInt64,
    chatgptAccountId  :Text,
    email             :Text,
    planType          :Text
  );

  refreshAccountToken @2 (
    accountId :Text
  ) -> (
    success      :Bool,
    message      :Text,
    accessToken  :Text,
    refreshToken :Text,
    idToken      :Text,
    expiresIn    :UInt64
  );

  revokeToken @3 (
    accountId :Text
  ) -> (success :Bool, message :Text);
}
