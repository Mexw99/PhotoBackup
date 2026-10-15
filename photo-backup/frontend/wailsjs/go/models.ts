export namespace backup {
	
	export class ImageFile {
	    name: string;
	    path: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new ImageFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	    }
	}

}

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

