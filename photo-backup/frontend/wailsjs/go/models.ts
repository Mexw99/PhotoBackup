export namespace backup {
	
	export class BackupResult {
	    originalName: string;
	    originalPath: string;
	    backupPath: string;
	    fileSize: number;
	    success: boolean;
	    skipped: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.originalName = source["originalName"];
	        this.originalPath = source["originalPath"];
	        this.backupPath = source["backupPath"];
	        this.fileSize = source["fileSize"];
	        this.success = source["success"];
	        this.skipped = source["skipped"];
	        this.error = source["error"];
	    }
	}
	export class BackupSummary {
	    total: number;
	    success: number;
	    skipped: number;
	    failed: number;
	    results: BackupResult[];
	
	    static createFrom(source: any = {}) {
	        return new BackupSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.success = source["success"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.results = this.convertValues(source["results"], BackupResult);
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
	
	export class BackupSettings {
	    sourcePath: string;
	    destPath: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourcePath = source["sourcePath"];
	        this.destPath = source["destPath"];
	    }
	}
	export class PhotoSummary {
	    id: number;
	    originalName: string;
	    backupPath: string;
	    backedUpAt: string;
	    thumbnail: string;
	    colorTag: string;
	
	    static createFrom(source: any = {}) {
	        return new PhotoSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.originalName = source["originalName"];
	        this.backupPath = source["backupPath"];
	        this.backedUpAt = source["backedUpAt"];
	        this.thumbnail = source["thumbnail"];
	        this.colorTag = source["colorTag"];
	    }
	}
	export class TagCloudItem {
	    id: number;
	    name: string;
	    photoCount: number;
	
	    static createFrom(source: any = {}) {
	        return new TagCloudItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.photoCount = source["photoCount"];
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
	export class TaggedPhotoSummary {
	    id: number;
	    originalName: string;
	    backupPath: string;
	    backedUpAt: string;
	    thumbnail: string;
	    colorTag: string;
	    tags: TagSummary[];
	
	    static createFrom(source: any = {}) {
	        return new TaggedPhotoSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.originalName = source["originalName"];
	        this.backupPath = source["backupPath"];
	        this.backedUpAt = source["backedUpAt"];
	        this.thumbnail = source["thumbnail"];
	        this.colorTag = source["colorTag"];
	        this.tags = this.convertValues(source["tags"], TagSummary);
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

}

