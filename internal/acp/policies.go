package acp

// RuntimePolicies is the machine-readable contract returned by acp_session
// info. Values are derived from the same constants that enforce the runtime,
// keeping client guidance and server behavior in one source of truth.
type RuntimePolicies struct {
	Context      ContextPolicy     `json:"context_policy"`
	Events       EventPolicy       `json:"event_policy"`
	Interactions InteractionPolicy `json:"interaction_policy"`
	Steering     SteeringPolicy    `json:"steering_policy"`
}

type ContextPolicy struct {
	HistoryOwner                string `json:"history_owner"`
	AgentDockPersistsTranscript bool   `json:"agentdock_persists_transcript"`
	AgentDockReplaysTranscript  bool   `json:"agentdock_replays_transcript"`
	RestartRequiresExplicitLoad bool   `json:"restart_requires_explicit_load"`
}

type EventPolicy struct {
	Incremental       bool   `json:"incremental"`
	Cursor            string `json:"cursor"`
	MaxEventsPerPage  int    `json:"max_events_per_page"`
	MaxRetainedEvents int    `json:"max_retained_events"`
	MaxRetainedBytes  int    `json:"max_retained_bytes"`
	MaxUpdateBytes    int    `json:"max_update_bytes"`
}

type InteractionPolicy struct {
	MemoryOnly            bool `json:"memory_only"`
	MaxOptions            int  `json:"max_options"`
	MaxToolCallBytes      int  `json:"max_tool_call_bytes"`
	MaxPendingGlobal      int  `json:"max_pending_global"`
	MaxPendingPerSession  int  `json:"max_pending_per_session"`
	AlwaysOptionsFiltered bool `json:"always_options_filtered"`
}

type SteeringPolicy struct {
	NativeWhenSupported             bool `json:"native_when_supported"`
	IdlePromptRequiredStartsRun     bool `json:"idle_prompt_required_starts_run"`
	CompatibilityFallbackReturnsRun bool `json:"compatibility_fallback_returns_run"`
}

func CurrentPolicies() RuntimePolicies {
	return RuntimePolicies{
		Context: ContextPolicy{
			HistoryOwner: "adapter", AgentDockPersistsTranscript: false,
			AgentDockReplaysTranscript: false, RestartRequiresExplicitLoad: true,
		},
		Events: EventPolicy{
			Incremental: true, Cursor: "next_seq", MaxEventsPerPage: 200,
			MaxRetainedEvents: maxEventCount, MaxRetainedBytes: maxEventBytes,
			MaxUpdateBytes: maxEventUpdateBytes,
		},
		Interactions: InteractionPolicy{
			MemoryOnly: true, MaxOptions: maxPermissionOptions,
			MaxToolCallBytes:      maxPermissionToolCallBytes,
			MaxPendingGlobal:      maxPendingInteractions,
			MaxPendingPerSession:  maxPendingInteractionsPerSession,
			AlwaysOptionsFiltered: true,
		},
		Steering: SteeringPolicy{
			NativeWhenSupported: true, IdlePromptRequiredStartsRun: true,
			CompatibilityFallbackReturnsRun: true,
		},
	}
}
