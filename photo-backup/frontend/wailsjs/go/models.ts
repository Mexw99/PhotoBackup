export namespace main {
	
	export class PhotoSummary {
	    id: number;
	    originalName: string;
	    backupPath: string;
	    backedUpAt: string;
	
	    static createFrom(source: any = {}) {
	        return new PhotoSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.originalName = source["originalName"];
	        this.backupPath = source["backupPath"];
	        this.backedUpAt = source["backedUpAt"];
	    }
	}
	export class TagSummary {
	    id: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new TagSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}

}

