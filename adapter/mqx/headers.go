package mqx

// 消息头约定。事件层写入信封字段，适配器只透传，不解释业务语义。
const (
	// HeaderEventID 是事件唯一标识，与 Message.ID 同值。
	HeaderEventID = "mqx.event_id"
	// HeaderEventType 是事件类型，与 Message.RoutingKey 同值。
	HeaderEventType = "mqx.event_type"
	// HeaderSchemaVersion 是事件负载的契约版本。
	HeaderSchemaVersion = "mqx.schema_version"
	// HeaderOccurredAt 是事件发生时间（RFC3339Nano）。
	HeaderOccurredAt = "mqx.occurred_at"
	// HeaderProducer 是生产者标识，便于排障与归属。
	HeaderProducer = "mqx.producer"

	// 死信元数据：仅在进入死信时追加。
	HeaderOriginalDestination = "mqx.original_destination"
	HeaderOriginalRoutingKey  = "mqx.original_routing_key"
	HeaderOriginalGroup       = "mqx.original_group"
	HeaderAttempts            = "mqx.attempts"
	HeaderError               = "mqx.error"
	HeaderFailedAt            = "mqx.failed_at"
)
