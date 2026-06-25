export namespace main {
	
	export class AdvancedProxyAppConfig {
	    enabled: boolean;
	    basePath: string;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyAppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.basePath = source["basePath"];
	    }
	}
	export class AntiPoisonStringProtectionConfig {
	    enabled: boolean;
	    rules: string[];
	
	    static createFrom(source: any = {}) {
	        return new AntiPoisonStringProtectionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.rules = source["rules"];
	    }
	}
	export class AntiPoisonRandomizationConfig {
	    enabled: boolean;
	    strategyPoolSize: number;
	    minPhraseVariantsPerStrategy: number;
	    randomInsertionPoints: boolean;
	    minFakeToolcalls: number;
	    requirePerToolTypeMarker: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AntiPoisonRandomizationConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.strategyPoolSize = source["strategyPoolSize"];
	        this.minPhraseVariantsPerStrategy = source["minPhraseVariantsPerStrategy"];
	        this.randomInsertionPoints = source["randomInsertionPoints"];
	        this.minFakeToolcalls = source["minFakeToolcalls"];
	        this.requirePerToolTypeMarker = source["requirePerToolTypeMarker"];
	    }
	}
	export class AntiPoisonConfig {
	    enabled: boolean;
	    strictMode: boolean;
	    failureMode: string;
	    strategyPrompt: string;
	    algorithmPrompt: string;
	    randomization: AntiPoisonRandomizationConfig;
	    stringProtection: AntiPoisonStringProtectionConfig;
	
	    static createFrom(source: any = {}) {
	        return new AntiPoisonConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.strictMode = source["strictMode"];
	        this.failureMode = source["failureMode"];
	        this.strategyPrompt = source["strategyPrompt"];
	        this.algorithmPrompt = source["algorithmPrompt"];
	        this.randomization = this.convertValues(source["randomization"], AntiPoisonRandomizationConfig);
	        this.stringProtection = this.convertValues(source["stringProtection"], AntiPoisonStringProtectionConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OptimizerConfig {
	    enabled: boolean;
	    thinkingOptimizer: boolean;
	    cacheInjection: boolean;
	    cacheTtl: string;
	
	    static createFrom(source: any = {}) {
	        return new OptimizerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.thinkingOptimizer = source["thinkingOptimizer"];
	        this.cacheInjection = source["cacheInjection"];
	        this.cacheTtl = source["cacheTtl"];
	    }
	}
	export class RectifierConfig {
	    enabled: boolean;
	    requestThinkingSignature: boolean;
	    requestThinkingBudget: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RectifierConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.requestThinkingSignature = source["requestThinkingSignature"];
	        this.requestThinkingBudget = source["requestThinkingBudget"];
	    }
	}
	export class int {
	
	
	    static createFrom(source: any = {}) {
	        return new int(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class HighAvailabilityRPMConfig {
	    global: number;
	    providers: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new HighAvailabilityRPMConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.global = source["global"];
	        this.providers = source["providers"];
	    }
	}
	export class HighAvailabilityConfig {
	    enabled: boolean;
	    dynamicOptimizeQueue: boolean;
	    dispatchMode: string;
	    rpm: HighAvailabilityRPMConfig;
	
	    static createFrom(source: any = {}) {
	        return new HighAvailabilityConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.dynamicOptimizeQueue = source["dynamicOptimizeQueue"];
	        this.dispatchMode = source["dispatchMode"];
	        this.rpm = this.convertValues(source["rpm"], HighAvailabilityRPMConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AppFailoverConfig {
	    appType: string;
	    enabled: boolean;
	    autoFailoverEnabled: boolean;
	    maxRetries: number;
	    streamingFirstByteTimeout: number;
	    streamingIdleTimeout: number;
	    nonStreamingTimeout: number;
	    circuitFailureThreshold: number;
	    circuitSuccessThreshold: number;
	    circuitTimeoutSeconds: number;
	    circuitErrorRateThreshold: number;
	    circuitMinRequests: number;
	
	    static createFrom(source: any = {}) {
	        return new AppFailoverConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appType = source["appType"];
	        this.enabled = source["enabled"];
	        this.autoFailoverEnabled = source["autoFailoverEnabled"];
	        this.maxRetries = source["maxRetries"];
	        this.streamingFirstByteTimeout = source["streamingFirstByteTimeout"];
	        this.streamingIdleTimeout = source["streamingIdleTimeout"];
	        this.nonStreamingTimeout = source["nonStreamingTimeout"];
	        this.circuitFailureThreshold = source["circuitFailureThreshold"];
	        this.circuitSuccessThreshold = source["circuitSuccessThreshold"];
	        this.circuitTimeoutSeconds = source["circuitTimeoutSeconds"];
	        this.circuitErrorRateThreshold = source["circuitErrorRateThreshold"];
	        this.circuitMinRequests = source["circuitMinRequests"];
	    }
	}
	export class ClaudeProxyCompatConfig {
	    enabled: boolean;
	    basePath: string;
	    defaultModel: string;
	    providers: AdvancedProxyProvider[];
	
	    static createFrom(source: any = {}) {
	        return new ClaudeProxyCompatConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.basePath = source["basePath"];
	        this.defaultModel = source["defaultModel"];
	        this.providers = this.convertValues(source["providers"], AdvancedProxyProvider);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class checkUserAgentMapping {
	    ModelContains: string;
	    TargetUA: string;
	
	    static createFrom(source: any = {}) {
	        return new checkUserAgentMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ModelContains = source["ModelContains"];
	        this.TargetUA = source["TargetUA"];
	    }
	}
	export class AdvancedProxyProvider {
	    id: string;
	    rowKey?: string;
	    name: string;
	    baseUrl: string;
	    apiKey: string;
	    model: string;
	    apiFormat: string;
	    apiKeyField: string;
	    enabled: boolean;
	    sortIndex: number;
	    sourceType?: string;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyProvider(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.rowKey = source["rowKey"];
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	        this.apiFormat = source["apiFormat"];
	        this.apiKeyField = source["apiKeyField"];
	        this.enabled = source["enabled"];
	        this.sortIndex = source["sortIndex"];
	        this.sourceType = source["sourceType"];
	    }
	}
	export class AdvancedProxyQueueConfig {
	    inheritGlobal: boolean;
	    providers: AdvancedProxyProvider[];
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyQueueConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inheritGlobal = source["inheritGlobal"];
	        this.providers = this.convertValues(source["providers"], AdvancedProxyProvider);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AdvancedProxyQueuesConfig {
	    global: AdvancedProxyQueueConfig;
	    claude: AdvancedProxyQueueConfig;
	    codex: AdvancedProxyQueueConfig;
	    opencode: AdvancedProxyQueueConfig;
	    openclaw: AdvancedProxyQueueConfig;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyQueuesConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.global = this.convertValues(source["global"], AdvancedProxyQueueConfig);
	        this.claude = this.convertValues(source["claude"], AdvancedProxyQueueConfig);
	        this.codex = this.convertValues(source["codex"], AdvancedProxyQueueConfig);
	        this.opencode = this.convertValues(source["opencode"], AdvancedProxyQueueConfig);
	        this.openclaw = this.convertValues(source["openclaw"], AdvancedProxyQueueConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AdvancedProxyConfig {
	    enabled: boolean;
	    debugLogging: boolean;
	    listenHost: string;
	    listenPort: number;
	    queues: AdvancedProxyQueuesConfig;
	    userAgentMappings: checkUserAgentMapping[];
	    claude: ClaudeProxyCompatConfig;
	    codex: AdvancedProxyAppConfig;
	    opencode: AdvancedProxyAppConfig;
	    openclaw: AdvancedProxyAppConfig;
	    failover: AppFailoverConfig;
	    highAvailability: HighAvailabilityConfig;
	    rectifier: RectifierConfig;
	    optimizer: OptimizerConfig;
	    antiPoison: AntiPoisonConfig;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.debugLogging = source["debugLogging"];
	        this.listenHost = source["listenHost"];
	        this.listenPort = source["listenPort"];
	        this.queues = this.convertValues(source["queues"], AdvancedProxyQueuesConfig);
	        this.userAgentMappings = this.convertValues(source["userAgentMappings"], checkUserAgentMapping);
	        this.claude = this.convertValues(source["claude"], ClaudeProxyCompatConfig);
	        this.codex = this.convertValues(source["codex"], AdvancedProxyAppConfig);
	        this.opencode = this.convertValues(source["opencode"], AdvancedProxyAppConfig);
	        this.openclaw = this.convertValues(source["openclaw"], AdvancedProxyAppConfig);
	        this.failover = this.convertValues(source["failover"], AppFailoverConfig);
	        this.highAvailability = this.convertValues(source["highAvailability"], HighAvailabilityConfig);
	        this.rectifier = this.convertValues(source["rectifier"], RectifierConfig);
	        this.optimizer = this.convertValues(source["optimizer"], OptimizerConfig);
	        this.antiPoison = this.convertValues(source["antiPoison"], AntiPoisonConfig);
	        this.updatedAt = source["updatedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class AdvancedProxyProviderRoutingState {
	    providerId: string;
	    providerRowKey: string;
	    providerName: string;
	    appTypes: string[];
	    activeCount: number;
	    routeKind: string;
	    status: string;
	    targetUrl: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyProviderRoutingState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providerId = source["providerId"];
	        this.providerRowKey = source["providerRowKey"];
	        this.providerName = source["providerName"];
	        this.appTypes = source["appTypes"];
	        this.activeCount = source["activeCount"];
	        this.routeKind = source["routeKind"];
	        this.status = source["status"];
	        this.targetUrl = source["targetUrl"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	
	
	export class antiPoisonOperationRecord {
	    id: string;
	    time: string;
	    stage: string;
	    channel: string;
	    rule: string;
	    path: string;
	    before: string;
	    after: string;
	    context: string;
	    count: number;
	    route: string;
	    provider: string;
	    blocked: boolean;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new antiPoisonOperationRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = source["time"];
	        this.stage = source["stage"];
	        this.channel = source["channel"];
	        this.rule = source["rule"];
	        this.path = source["path"];
	        this.before = source["before"];
	        this.after = source["after"];
	        this.context = source["context"];
	        this.count = source["count"];
	        this.route = source["route"];
	        this.provider = source["provider"];
	        this.blocked = source["blocked"];
	        this.reason = source["reason"];
	    }
	}
	export class advancedProxyObservedItem {
	    type: string;
	    name?: string;
	    argumentsPreview?: string;
	    textPreview?: string;
	    rawPreview?: string;
	
	    static createFrom(source: any = {}) {
	        return new advancedProxyObservedItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.name = source["name"];
	        this.argumentsPreview = source["argumentsPreview"];
	        this.textPreview = source["textPreview"];
	        this.rawPreview = source["rawPreview"];
	    }
	}
	export class AdvancedProxyRequestRouteStep {
	    route: string;
	    source?: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyRequestRouteStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.route = source["route"];
	        this.source = source["source"];
	        this.status = source["status"];
	    }
	}
	export class AdvancedProxyRequestRecord {
	    id: string;
	    recordedAt: string;
	    appType: string;
	    clientRoute: string;
	    inboundEndpoint: string;
	    outboundRoute: string;
	    routeTrace?: AdvancedProxyRequestRouteStep[];
	    providerId: string;
	    providerRowKey: string;
	    providerName: string;
	    providerKeyPreview: string;
	    model: string;
	    stream: boolean;
	    statusCode: number;
	    durationMs: number;
	    ttftMs?: number;
	    latencyMs?: number;
	    inputTokens?: number;
	    outputTokens?: number;
	    tps?: number;
	    upstreamUrl: string;
	    upstreamEndpoint: string;
	    errorDetail: string;
	    source: string;
	    requestBody?: string;
	    antiPoisonPromptPreview?: string;
	    upstreamResponsePreview?: string;
	    upstreamResponseRaw?: string;
	    responsePreview?: string;
	    upstreamToolCalls?: string[];
	    upstreamToolArgsPreview?: string[];
	    upstreamAssistantPreview?: string;
	    upstreamLatestObserved?: advancedProxyObservedItem;
	    antiPoisonOps?: antiPoisonOperationRecord[];
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyRequestRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.recordedAt = source["recordedAt"];
	        this.appType = source["appType"];
	        this.clientRoute = source["clientRoute"];
	        this.inboundEndpoint = source["inboundEndpoint"];
	        this.outboundRoute = source["outboundRoute"];
	        this.routeTrace = this.convertValues(source["routeTrace"], AdvancedProxyRequestRouteStep);
	        this.providerId = source["providerId"];
	        this.providerRowKey = source["providerRowKey"];
	        this.providerName = source["providerName"];
	        this.providerKeyPreview = source["providerKeyPreview"];
	        this.model = source["model"];
	        this.stream = source["stream"];
	        this.statusCode = source["statusCode"];
	        this.durationMs = source["durationMs"];
	        this.ttftMs = source["ttftMs"];
	        this.latencyMs = source["latencyMs"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.tps = source["tps"];
	        this.upstreamUrl = source["upstreamUrl"];
	        this.upstreamEndpoint = source["upstreamEndpoint"];
	        this.errorDetail = source["errorDetail"];
	        this.source = source["source"];
	        this.requestBody = source["requestBody"];
	        this.antiPoisonPromptPreview = source["antiPoisonPromptPreview"];
	        this.upstreamResponsePreview = source["upstreamResponsePreview"];
	        this.upstreamResponseRaw = source["upstreamResponseRaw"];
	        this.responsePreview = source["responsePreview"];
	        this.upstreamToolCalls = source["upstreamToolCalls"];
	        this.upstreamToolArgsPreview = source["upstreamToolArgsPreview"];
	        this.upstreamAssistantPreview = source["upstreamAssistantPreview"];
	        this.upstreamLatestObserved = this.convertValues(source["upstreamLatestObserved"], advancedProxyObservedItem);
	        this.antiPoisonOps = this.convertValues(source["antiPoisonOps"], antiPoisonOperationRecord);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class AdvancedProxyRoutingState {
	    appType: string;
	    providerId: string;
	    providerRowKey: string;
	    providerName: string;
	    routeKind: string;
	    status: string;
	    targetUrl: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyRoutingState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appType = source["appType"];
	        this.providerId = source["providerId"];
	        this.providerRowKey = source["providerRowKey"];
	        this.providerName = source["providerName"];
	        this.routeKind = source["routeKind"];
	        this.status = source["status"];
	        this.targetUrl = source["targetUrl"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class AdvancedProxyRoutingSnapshot {
	    apps: Record<string, AdvancedProxyRoutingState>;
	    providers: Record<string, AdvancedProxyProviderRoutingState>;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyRoutingSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apps = this.convertValues(source["apps"], AdvancedProxyRoutingState, true);
	        this.providers = this.convertValues(source["providers"], AdvancedProxyProviderRoutingState, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	export class AppUpdateAsset {
	    name: string;
	    browserDownloadUrl: string;
	    size: number;
	    contentType: string;
	
	    static createFrom(source: any = {}) {
	        return new AppUpdateAsset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.browserDownloadUrl = source["browserDownloadUrl"];
	        this.size = source["size"];
	        this.contentType = source["contentType"];
	    }
	}
	export class AppUpdateDownloadSnapshot {
	    active: boolean;
	    stage: string;
	    latestTag: string;
	    fileName: string;
	    downloadUrl: string;
	    savedPath: string;
	    totalBytes: number;
	    receivedBytes: number;
	    percent: number;
	    message: string;
	    error: string;
	    startedAt: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new AppUpdateDownloadSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.stage = source["stage"];
	        this.latestTag = source["latestTag"];
	        this.fileName = source["fileName"];
	        this.downloadUrl = source["downloadUrl"];
	        this.savedPath = source["savedPath"];
	        this.totalBytes = source["totalBytes"];
	        this.receivedBytes = source["receivedBytes"];
	        this.percent = source["percent"];
	        this.message = source["message"];
	        this.error = source["error"];
	        this.startedAt = source["startedAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class AppUpdateReleaseInfo {
	    latestTag: string;
	    latestVersion: string;
	    htmlUrl: string;
	    body: string;
	    targetOs: string;
	    targetArch: string;
	    asset?: AppUpdateAsset;
	
	    static createFrom(source: any = {}) {
	        return new AppUpdateReleaseInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latestTag = source["latestTag"];
	        this.latestVersion = source["latestVersion"];
	        this.htmlUrl = source["htmlUrl"];
	        this.body = source["body"];
	        this.targetOs = source["targetOs"];
	        this.targetArch = source["targetArch"];
	        this.asset = this.convertValues(source["asset"], AppUpdateAsset);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BridgeImportRecord {
	    id: string;
	    receivedAt: string;
	    remoteAddr: string;
	    type: string;
	    sourceUrl: string;
	    sourceOrigin: string;
	    title: string;
	    userAgent: string;
	    siteType: string;
	    resolvedUser: string;
	    tokenPreview: string;
	    tokenCount: number;
	    tokenEndpoint: string;
	    ready: boolean;
	    readyReason: string;
	    payload: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new BridgeImportRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.receivedAt = source["receivedAt"];
	        this.remoteAddr = source["remoteAddr"];
	        this.type = source["type"];
	        this.sourceUrl = source["sourceUrl"];
	        this.sourceOrigin = source["sourceOrigin"];
	        this.title = source["title"];
	        this.userAgent = source["userAgent"];
	        this.siteType = source["siteType"];
	        this.resolvedUser = source["resolvedUser"];
	        this.tokenPreview = source["tokenPreview"];
	        this.tokenCount = source["tokenCount"];
	        this.tokenEndpoint = source["tokenEndpoint"];
	        this.ready = source["ready"];
	        this.readyReason = source["readyReason"];
	        this.payload = source["payload"];
	    }
	}
	export class BridgeImportSnapshot {
	    records: BridgeImportRecord[];
	    totalCount: number;
	    readyCount: number;
	    lastReceivedAt: string;
	    lastStoredAt: string;
	    logPath: string;
	    serverUrl: string;
	    lastLogs: string[];
	    sessionActive: boolean;
	    clientReady: boolean;
	    lastClientPing: string;
	
	    static createFrom(source: any = {}) {
	        return new BridgeImportSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.records = this.convertValues(source["records"], BridgeImportRecord);
	        this.totalCount = source["totalCount"];
	        this.readyCount = source["readyCount"];
	        this.lastReceivedAt = source["lastReceivedAt"];
	        this.lastStoredAt = source["lastStoredAt"];
	        this.logPath = source["logPath"];
	        this.serverUrl = source["serverUrl"];
	        this.lastLogs = source["lastLogs"];
	        this.sessionActive = source["sessionActive"];
	        this.clientReady = source["clientReady"];
	        this.lastClientPing = source["lastClientPing"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChromeProfileAccountInfoInput {
	    id: any;
	    access_token: string;
	
	    static createFrom(source: any = {}) {
	        return new ChromeProfileAccountInfoInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.access_token = source["access_token"];
	    }
	}
	export class ChromeProfileAccount {
	    id: string;
	    site_name: string;
	    site_url: string;
	    site_type: string;
	    api_key: string;
	    account_info: ChromeProfileAccountInfoInput;
	
	    static createFrom(source: any = {}) {
	        return new ChromeProfileAccount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.site_name = source["site_name"];
	        this.site_url = source["site_url"];
	        this.site_type = source["site_type"];
	        this.api_key = source["api_key"];
	        this.account_info = this.convertValues(source["account_info"], ChromeProfileAccountInfoInput);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ChromeProfileTokenRequest {
	    accounts: ChromeProfileAccount[];
	
	    static createFrom(source: any = {}) {
	        return new ChromeProfileTokenRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accounts = this.convertValues(source["accounts"], ChromeProfileAccount);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChromeProfileTokenResult {
	    id: string;
	    site_name: string;
	    site_url: string;
	    tokens: any[];
	    error?: string;
	    resolved_access_token?: string;
	    resolved_user_id?: string;
	    storage_fields?: string[];
	    storage_origin?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChromeProfileTokenResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.site_name = source["site_name"];
	        this.site_url = source["site_url"];
	        this.tokens = source["tokens"];
	        this.error = source["error"];
	        this.resolved_access_token = source["resolved_access_token"];
	        this.resolved_user_id = source["resolved_user_id"];
	        this.storage_fields = source["storage_fields"];
	        this.storage_origin = source["storage_origin"];
	    }
	}
	export class ChromeProfileTokenResponse {
	    results: ChromeProfileTokenResult[];
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ChromeProfileTokenResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.results = this.convertValues(source["results"], ChromeProfileTokenResult);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class CircuitBreakerStats {
	    state: string;
	    consecutiveFailures: number;
	    consecutiveSuccesses: number;
	    totalRequests: number;
	    failedRequests: number;
	
	    static createFrom(source: any = {}) {
	        return new CircuitBreakerStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.consecutiveFailures = source["consecutiveFailures"];
	        this.consecutiveSuccesses = source["consecutiveSuccesses"];
	        this.totalRequests = source["totalRequests"];
	        this.failedRequests = source["failedRequests"];
	    }
	}
	
	export class DesktopLogContent {
	    path: string;
	    name: string;
	    content: string;
	    size: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new DesktopLogContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.size = source["size"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class DesktopLogFileInfo {
	    groupKey: string;
	    groupLabel: string;
	    sourceKey: string;
	    sourceLabel: string;
	    name: string;
	    path: string;
	    size: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new DesktopLogFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupKey = source["groupKey"];
	        this.groupLabel = source["groupLabel"];
	        this.sourceKey = source["sourceKey"];
	        this.sourceLabel = source["sourceLabel"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class DesktopLogSnapshot {
	    files: DesktopLogFileInfo[];
	
	    static createFrom(source: any = {}) {
	        return new DesktopLogSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], DesktopLogFileInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ExtensionImportResult {
	    sourcePath: string;
	    storageKey: string;
	    accountCount: number;
	    payload: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourcePath = source["sourcePath"];
	        this.storageKey = source["storageKey"];
	        this.accountCount = source["accountCount"];
	        this.payload = source["payload"];
	    }
	}
	export class FailoverQueueItem {
	    providerId: string;
	    providerName: string;
	    sortIndex: number;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FailoverQueueItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providerId = source["providerId"];
	        this.providerName = source["providerName"];
	        this.sortIndex = source["sortIndex"];
	        this.enabled = source["enabled"];
	    }
	}
	
	
	export class ManagedAppConfigAppliedFile {
	    appId: string;
	    fileId: string;
	    path: string;
	    backupPath: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigAppliedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.fileId = source["fileId"];
	        this.path = source["path"];
	        this.backupPath = source["backupPath"];
	    }
	}
	export class ManagedAppConfigWrite {
	    appId: string;
	    fileId: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigWrite(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.fileId = source["fileId"];
	        this.content = source["content"];
	    }
	}
	export class ManagedAppConfigApplyRequest {
	    files: ManagedAppConfigWrite[];
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigApplyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], ManagedAppConfigWrite);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ManagedAppConfigApplyResult {
	    applied: ManagedAppConfigAppliedFile[];
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.applied = this.convertValues(source["applied"], ManagedAppConfigAppliedFile);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ManagedAppConfigFile {
	    appId: string;
	    appName: string;
	    fileId: string;
	    label: string;
	    path: string;
	    exists: boolean;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.appName = source["appName"];
	        this.fileId = source["fileId"];
	        this.label = source["label"];
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.content = source["content"];
	    }
	}
	export class ManagedAppConfigSnapshot {
	    files: ManagedAppConfigFile[];
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], ManagedAppConfigFile);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class OutboundProxyConfig {
	    mode: string;
	    customUrl: string;
	    updatedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new OutboundProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.customUrl = source["customUrl"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class PortableDataPackageResult {
	    backupDir: string;
	    runtimeSourceDir: string;
	    runtimeBackupDir: string;
	    localStorageBackupPath: string;
	    localStorageKeyCount: number;
	
	    static createFrom(source: any = {}) {
	        return new PortableDataPackageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backupDir = source["backupDir"];
	        this.runtimeSourceDir = source["runtimeSourceDir"];
	        this.runtimeBackupDir = source["runtimeBackupDir"];
	        this.localStorageBackupPath = source["localStorageBackupPath"];
	        this.localStorageKeyCount = source["localStorageKeyCount"];
	    }
	}
	export class PortableDataUnpackResult {
	    backupDir: string;
	    runtimeBackupDir: string;
	    localStorageBackupPath: string;
	    localStorageJson: string;
	    localStorageKeyCount: number;
	
	    static createFrom(source: any = {}) {
	        return new PortableDataUnpackResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backupDir = source["backupDir"];
	        this.runtimeBackupDir = source["runtimeBackupDir"];
	        this.localStorageBackupPath = source["localStorageBackupPath"];
	        this.localStorageJson = source["localStorageJson"];
	        this.localStorageKeyCount = source["localStorageKeyCount"];
	    }
	}
	
	
	
	
	export class desktopProfileAssistOpenRequest {
	    siteName: string;
	    siteUrl: string;
	    siteType: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopProfileAssistOpenRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.siteName = source["siteName"];
	        this.siteUrl = source["siteUrl"];
	        this.siteType = source["siteType"];
	    }
	}
	export class fetchKeysProgressSnapshot {
	    active: boolean;
	    stage: string;
	    detail: string;
	    total: number;
	    completed: number;
	    successSites: number;
	    lastSiteName: string;
	    startedAt: number;
	    lastUpdatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new fetchKeysProgressSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.stage = source["stage"];
	        this.detail = source["detail"];
	        this.total = source["total"];
	        this.completed = source["completed"];
	        this.successSites = source["successSites"];
	        this.lastSiteName = source["lastSiteName"];
	        this.startedAt = source["startedAt"];
	        this.lastUpdatedAt = source["lastUpdatedAt"];
	    }
	}
	export class sidebarWindowBounds {
	    Width: number;
	    Height: number;
	    X: number;
	    Y: number;
	
	    static createFrom(source: any = {}) {
	        return new sidebarWindowBounds(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Width = source["Width"];
	        this.Height = source["Height"];
	        this.X = source["X"];
	        this.Y = source["Y"];
	    }
	}

}

