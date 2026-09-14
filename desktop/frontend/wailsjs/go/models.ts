export namespace engine {
	
	export class EditView {
	    editIndex: number;
	    depth: number;
	    hasChildren: boolean;
	    descendantCount: number;
	
	    static createFrom(source: any = {}) {
	        return new EditView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.editIndex = source["editIndex"];
	        this.depth = source["depth"];
	        this.hasChildren = source["hasChildren"];
	        this.descendantCount = source["descendantCount"];
	    }
	}
	export class ReconciliationCandidate {
	    nodeId: string;
	    nodeGlobalId: string;
	    originalParentId?: string;
	    originalParentGlobalId?: string;
	    currentParentId?: string;
	    currentParentGlobalId?: string;
	    originalPath?: string;
	    currentPath?: string;
	    canReconcile: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new ReconciliationCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeId = source["nodeId"];
	        this.nodeGlobalId = source["nodeGlobalId"];
	        this.originalParentId = source["originalParentId"];
	        this.originalParentGlobalId = source["originalParentGlobalId"];
	        this.currentParentId = source["currentParentId"];
	        this.currentParentGlobalId = source["currentParentGlobalId"];
	        this.originalPath = source["originalPath"];
	        this.currentPath = source["currentPath"];
	        this.canReconcile = source["canReconcile"];
	        this.reason = source["reason"];
	    }
	}
	export class structuralASTNode {
	    id: string;
	    globalId: string;
	    originalPath?: string;
	    currentPath?: string;
	    originalParentId?: string;
	    currentParentId?: string;
	    originalParentGlobalId?: string;
	    currentParentGlobalId?: string;
	    originalField?: string;
	    currentField?: string;
	    originalIndex?: number;
	    currentIndex?: number;
	    kind: string;
	    value?: string;
	    field?: string;
	    index?: number;
	    children?: structuralASTNode[];
	
	    static createFrom(source: any = {}) {
	        return new structuralASTNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.globalId = source["globalId"];
	        this.originalPath = source["originalPath"];
	        this.currentPath = source["currentPath"];
	        this.originalParentId = source["originalParentId"];
	        this.currentParentId = source["currentParentId"];
	        this.originalParentGlobalId = source["originalParentGlobalId"];
	        this.currentParentGlobalId = source["currentParentGlobalId"];
	        this.originalField = source["originalField"];
	        this.currentField = source["currentField"];
	        this.originalIndex = source["originalIndex"];
	        this.currentIndex = source["currentIndex"];
	        this.kind = source["kind"];
	        this.value = source["value"];
	        this.field = source["field"];
	        this.index = source["index"];
	        this.children = this.convertValues(source["children"], structuralASTNode);
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
	export class structuralEdit {
	    index: number;
	    kind: string;
	    nodeId: string;
	    nodeGlobalId?: string;
	    sourceGlobalId?: string;
	    nodeKind: string;
	    parentId?: string;
	    parentGlobalId?: string;
	    parentKind?: string;
	    field?: string;
	    position: number;
	    value?: string;
	    node?: structuralASTNode;
	
	    static createFrom(source: any = {}) {
	        return new structuralEdit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.kind = source["kind"];
	        this.nodeId = source["nodeId"];
	        this.nodeGlobalId = source["nodeGlobalId"];
	        this.sourceGlobalId = source["sourceGlobalId"];
	        this.nodeKind = source["nodeKind"];
	        this.parentId = source["parentId"];
	        this.parentGlobalId = source["parentGlobalId"];
	        this.parentKind = source["parentKind"];
	        this.field = source["field"];
	        this.position = source["position"];
	        this.value = source["value"];
	        this.node = this.convertValues(source["node"], structuralASTNode);
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

export namespace explorer {
	
	export class Declaration {
	    kind: string;
	    name: string;
	    receiver?: string;
	    line: number;
	    endLine: number;
	    exported: boolean;
	    children?: Declaration[];
	
	    static createFrom(source: any = {}) {
	        return new Declaration(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.receiver = source["receiver"];
	        this.line = source["line"];
	        this.endLine = source["endLine"];
	        this.exported = source["exported"];
	        this.children = this.convertValues(source["children"], Declaration);
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
	export class File {
	    name: string;
	    path: string;
	    declarations?: Declaration[];
	    imports?: string[];
	
	    static createFrom(source: any = {}) {
	        return new File(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.declarations = this.convertValues(source["declarations"], Declaration);
	        this.imports = source["imports"];
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
	export class ImportSummary {
	    path: string;
	    name: string;
	    directory: string;
	    files?: File[];
	    declarations?: Declaration[];
	
	    static createFrom(source: any = {}) {
	        return new ImportSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.directory = source["directory"];
	        this.files = this.convertValues(source["files"], File);
	        this.declarations = this.convertValues(source["declarations"], Declaration);
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
	export class Package {
	    name: string;
	    directory: string;
	    fileCount: number;
	    files?: File[];
	    localImports?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Package(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.directory = source["directory"];
	        this.fileCount = source["fileCount"];
	        this.files = this.convertValues(source["files"], File);
	        this.localImports = source["localImports"];
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

export namespace main {
	
	export class ProgramCandidate {
	    ancestorId: string;
	    nodeId: string;
	    nodeGlobalId: string;
	    originalParentId?: string;
	    originalParentGlobalId?: string;
	    currentParentId?: string;
	    currentParentGlobalId?: string;
	    originalPath?: string;
	    currentPath?: string;
	    canReconcile: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgramCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ancestorId = source["ancestorId"];
	        this.nodeId = source["nodeId"];
	        this.nodeGlobalId = source["nodeGlobalId"];
	        this.originalParentId = source["originalParentId"];
	        this.originalParentGlobalId = source["originalParentGlobalId"];
	        this.currentParentId = source["currentParentId"];
	        this.currentParentGlobalId = source["currentParentGlobalId"];
	        this.originalPath = source["originalPath"];
	        this.currentPath = source["currentPath"];
	        this.canReconcile = source["canReconcile"];
	        this.reason = source["reason"];
	    }
	}
	export class ProgramSnapshot {
	    path: string;
	    source: string;
	    target: string;
	    working?: engine.structuralASTNode;
	    workingCode: string;
	    edits: engine.structuralEdit[];
	    status: string[];
	    valid: boolean;
	    diagnostics?: string[];
	    renderDiagnostics?: string[];
	    candidates?: ProgramCandidate[];
	    editViews: engine.EditView[];
	    exploration: boolean;
	    packages?: explorer.Package[];
	    packageName?: string;
	    packageDirectory: string;
	    fileName?: string;
	    filePath?: string;
	    declarations?: explorer.Declaration[];
	    fileSource?: string;
	    declarationName?: string;
	    declarationSource?: string;
	    localImports?: string[];
	    imports?: explorer.ImportSummary[];
	    importedFileName?: string;
	    importedSource?: string;
	    importedName?: string;
	    files?: explorer.File[];
	
	    static createFrom(source: any = {}) {
	        return new ProgramSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.source = source["source"];
	        this.target = source["target"];
	        this.working = this.convertValues(source["working"], engine.structuralASTNode);
	        this.workingCode = source["workingCode"];
	        this.edits = this.convertValues(source["edits"], engine.structuralEdit);
	        this.status = source["status"];
	        this.valid = source["valid"];
	        this.diagnostics = source["diagnostics"];
	        this.renderDiagnostics = source["renderDiagnostics"];
	        this.candidates = this.convertValues(source["candidates"], ProgramCandidate);
	        this.editViews = this.convertValues(source["editViews"], engine.EditView);
	        this.exploration = source["exploration"];
	        this.packages = this.convertValues(source["packages"], explorer.Package);
	        this.packageName = source["packageName"];
	        this.packageDirectory = source["packageDirectory"];
	        this.fileName = source["fileName"];
	        this.filePath = source["filePath"];
	        this.declarations = this.convertValues(source["declarations"], explorer.Declaration);
	        this.fileSource = source["fileSource"];
	        this.declarationName = source["declarationName"];
	        this.declarationSource = source["declarationSource"];
	        this.localImports = source["localImports"];
	        this.imports = this.convertValues(source["imports"], explorer.ImportSummary);
	        this.importedFileName = source["importedFileName"];
	        this.importedSource = source["importedSource"];
	        this.importedName = source["importedName"];
	        this.files = this.convertValues(source["files"], explorer.File);
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

