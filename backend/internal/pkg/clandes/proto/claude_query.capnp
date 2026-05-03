@0xd3e4f5a6b7c8d9e0;

using Go = import "/go.capnp";
$Go.package("proto");
$Go.import("github.com/Wei-Shaw/sub2api/internal/pkg/clandes/proto");

struct UsagePeriod {
  utilization @0 :Float64;
  resetsAt    @1 :Text;
}

struct ExtraUsage {
  isEnabled    @0 :Bool;
  monthlyLimit @1 :Int64;
  usedCredits  @2 :Int64;
  utilization  @3 :Float64;
}

interface ClaudeQueryService {
  getProfile @0 (accountId :Text) -> (
    success                :Bool,
    message                :Text,
    accountId              :Text,
    email                  :Text,
    displayName            :Text,
    createdAt              :Text,
    organizationId         :Text,
    organizationType       :Text,
    rateLimitTier          :Text,
    hasExtraUsageEnabled   :Bool,
    billingType            :Text,
    subscriptionCreatedAt  :Text
  );

  getUsage @1 (accountId :Text) -> (
    success       :Bool,
    message       :Text,
    fiveHour      :UsagePeriod,
    sevenDay      :UsagePeriod,
    sevenDaySonnet :UsagePeriod,
    extraUsage    :ExtraUsage
  );

  getRoles @2 (accountId :Text) -> (
    success          :Bool,
    message          :Text,
    organizationRole :Text,
    workspaceRole    :Text,
    organizationName :Text
  );
}
