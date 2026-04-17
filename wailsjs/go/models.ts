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
	    listenHost: string;
	    listenPort: number;
	    queues: AdvancedProxyQueuesConfig;
	    claude: ClaudeProxyCompatConfig;
	    codex: AdvancedProxyAppConfig;
	    opencode: AdvancedProxyAppConfig;
	    openclaw: AdvancedProxyAppConfig;
	    failover: AppFailoverConfig;
	    rectifier: RectifierConfig;
	    optimizer: OptimizerConfig;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.listenHost = source["listenHost"];
	        this.listenPort = source["listenPort"];
	        this.queues = this.convertValues(source["queues"], AdvancedProxyQueuesConfig);
	        this.claude = this.convertValues(source["claude"], ClaudeProxyCompatConfig);
	        this.codex = this.convertValues(source["codex"], AdvancedProxyAppConfig);
	        this.opencode = this.convertValues(source["opencode"], AdvancedProxyAppConfig);
	        this.openclaw = this.convertValues(source["openclaw"], AdvancedProxyAppConfig);
	        this.failover = this.convertValues(source["failover"], AppFailoverConfig);
	        this.rectifier = this.convertValues(source["rectifier"], RectifierConfig);
	        this.optimizer = this.convertValues(source["optimizer"], OptimizerConfig);
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
	
	    static createFrom(source: any = {}) {
	        return new AdvancedProxyRoutingSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apps = this.convertValues(source["apps"], AdvancedProxyRoutingState, true);
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

}

