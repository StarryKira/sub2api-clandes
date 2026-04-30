@0xb1c2d3e4f5a6b7c8;

using Go = import "/go.capnp";
$Go.package("proto");
$Go.import("github.com/Wei-Shaw/sub2api/internal/pkg/clandes/proto");

enum AccountType {
  unknown            @0;
  claudeSubscription @1;
  claudePayAsYouGo   @2;
  codexSubscription  @3;
  geminiSubscription @4;
  geminiApiKey       @5;
  awsBedrock         @6;
  codexApiKey        @7;
}

struct ClaudeTokenCredentials {
  accessToken  @0 :Text;
  refreshToken @1 :Text;
}

struct ClaudeApiKeyCredentials {
  apiKey  @0 :Text;
  baseUrl @1 :Text;
}

struct CodexOAuthCredentials {
  accessToken  @0 :Text;
  refreshToken @1 :Text;
  idToken      @2 :Text;
}

struct CodexApiKeyCredentials {
  apiKey  @0 :Text;
  baseUrl @1 :Text;
}

struct AccountCredentials {
  union {
    none              @0 :Void;
    claudeSubCreds    @1 :ClaudeTokenCredentials;
    claudeApiKeyCreds @2 :ClaudeApiKeyCredentials;
    codexOAuthCreds   @3 :CodexOAuthCredentials;
    codexApiKeyCreds  @4 :CodexApiKeyCredentials;
  }
}

struct AccountInfo {
  accountId   @0 :Text;
  accountType @1 :AccountType;
  proxyUrl    @2 :Text;
  version     @3 :Text;
  expiresIn   @4 :UInt64;
  configJson  @5 :Text;
  credentials @6 :AccountCredentials;
}

interface AccountService {
  registerAccount @0 (
    accountId   :Text,
    accountType :AccountType,
    proxyUrl    :Text,
    version     :Text,
    configJson  :Text,
    expiresIn   :UInt64,
    credentials :AccountCredentials
  ) -> (success :Bool, message :Text);

  updateAccount @1 (
    accountId         :Text,
    accountType       :AccountType,
    proxyUrl          :Text,
    configJson        :Text,
    version           :Text,
    claudeSubCreds    :ClaudeTokenCredentials,
    claudeApiKeyCreds :ClaudeApiKeyCredentials
  ) -> (success :Bool, message :Text);

  removeAccount @2 (accountId :Text) -> (success :Bool, message :Text);
  listAccounts  @3 ()               -> (accounts :List(AccountInfo));
}
