export namespace main {
	
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

}

