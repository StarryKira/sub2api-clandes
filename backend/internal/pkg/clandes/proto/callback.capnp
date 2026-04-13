@0xf5a6b7c8d9e0f1a2;

using Go = import "/go.capnp";
$Go.package("proto");
$Go.import("github.com/Wei-Shaw/sub2api/internal/pkg/clandes/proto");

struct UsageReport {
  accountId     @0 :Text;
  apiKey        @1 :Text;
  model         @2 :Text;
  inputTokens   @3 :UInt32;
  outputTokens  @4 :UInt32;
  thinkingLevel @5 :UInt32;
  statusCode    @6 :UInt32;
  durationMs    @7 :UInt64;
}

struct StreamChunkEvent {
  accountId  @0 :Text;
  chunkIndex @1 :UInt32;
  eventType  @2 :Text;
  data       @3 :Text;
}

# 后端实现此 capability，clandes-server 持有并主动调用
interface Router {
  # 同步路由查询：返回 accountId
  routeRequest @0 (
    requestId :Text,
    apiKey    :Text,
    model     :Text,
    endpoint  :Text
  ) -> (accountId :Text);

  # 用量上报（fire-and-forget）
  reportUsage @1 (requestId :Text, report :UsageReport) -> ();

  # Stream chunk 上报（fire-and-forget）
  reportChunk @2 (requestId :Text, chunk :StreamChunkEvent) -> ();
}

interface CallbackService {
  # 后端注册自己的 Router capability 到 clandes-server
  connect @0 (router :Router) -> ();
}
