export namespace engine {

	export class EditAncestor {
	    nodeId: string;
	    globalId?: string;
	    nodeKind: string;
	    field?: string;
	    value?: string;
	    startLine?: number;
	    endLine?: number;

	    static createFrom(source: any = {}) {
	        return new EditAncestor(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeId = source["nodeId"];
	        this.globalId = source["globalId"];
	        this.nodeKind = source["nodeKind"];
	        this.field = source["field"];
	        this.value = source["value"];
	        this.startLine = source["startLine"];
	        this.endLine = source["endLine"];
	    }
	}
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
	    startLine?: number;
	    endLine?: number;
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
	        this.startLine = source["startLine"];
	        this.endLine = source["endLine"];
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
	    ancestorIds?: string[];
	    ancestors?: EditAncestor[];
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
	        this.ancestorIds = source["ancestorIds"];
	        this.ancestors = this.convertValues(source["ancestors"], EditAncestor);
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

	export class Reference {
	    symbolId?: string;
	    name: string;
	    kind: string;
	    packagePath: string;
	    packageName?: string;
	    packageDirectory?: string;
	    filePath: string;
	    declaration: string;
	    line: number;
	    referenceLine?: number;

	    static createFrom(source: any = {}) {
	        return new Reference(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbolId = source["symbolId"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.packagePath = source["packagePath"];
	        this.packageName = source["packageName"];
	        this.packageDirectory = source["packageDirectory"];
	        this.filePath = source["filePath"];
	        this.declaration = source["declaration"];
	        this.line = source["line"];
	        this.referenceLine = source["referenceLine"];
	    }
	}
	export class Parameter {
	    name?: string;
	    type: string;

	    static createFrom(source: any = {}) {
	        return new Parameter(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class Declaration {
	    symbolId?: string;
	    kind: string;
	    name: string;
	    receiver?: string;
	    line: number;
	    endLine: number;
	    exported: boolean;
	    parameters?: Parameter[];
	    results?: Parameter[];
	    type?: string;
	    typeReferences?: Reference[];
	    children?: Declaration[];

	    static createFrom(source: any = {}) {
	        return new Declaration(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbolId = source["symbolId"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.receiver = source["receiver"];
	        this.line = source["line"];
	        this.endLine = source["endLine"];
	        this.exported = source["exported"];
	        this.parameters = this.convertValues(source["parameters"], Parameter);
	        this.results = this.convertValues(source["results"], Parameter);
	        this.type = source["type"];
	        this.typeReferences = this.convertValues(source["typeReferences"], Reference);
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
	export class Occurrence {
	    symbolId: string;
	    name: string;
	    startLine: number;
	    startColumn: number;
	    endLine: number;
	    endColumn: number;

	    static createFrom(source: any = {}) {
	        return new Occurrence(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbolId = source["symbolId"];
	        this.name = source["name"];
	        this.startLine = source["startLine"];
	        this.startColumn = source["startColumn"];
	        this.endLine = source["endLine"];
	        this.endColumn = source["endColumn"];
	    }
	}
	export class File {
	    name: string;
	    path: string;
	    declarations?: Declaration[];
	    imports?: string[];
	    occurrences?: Occurrence[];
	    samePackageReferences?: Reference[];

	    static createFrom(source: any = {}) {
	        return new File(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.declarations = this.convertValues(source["declarations"], Declaration);
	        this.imports = source["imports"];
	        this.occurrences = this.convertValues(source["occurrences"], Occurrence);
	        this.samePackageReferences = this.convertValues(source["samePackageReferences"], Reference);
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

	export class EditSummary {
	    index: number;
	    kind: string;
	    nodeId: string;
	    nodeGlobalId?: string;
	    sourceGlobalId?: string;
	    parentGlobalId?: string;
	    nodeKind: string;
	    parentId?: string;
	    parentKind?: string;
	    ancestorIds?: string[];
	    ancestors?: engine.EditAncestor[];
	    field?: string;
	    position: number;
	    value?: string;
	    startLine?: number;
	    endLine?: number;
	    status: string;

	    static createFrom(source: any = {}) {
	        return new EditSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.kind = source["kind"];
	        this.nodeId = source["nodeId"];
	        this.nodeGlobalId = source["nodeGlobalId"];
	        this.sourceGlobalId = source["sourceGlobalId"];
	        this.parentGlobalId = source["parentGlobalId"];
	        this.nodeKind = source["nodeKind"];
	        this.parentId = source["parentId"];
	        this.parentKind = source["parentKind"];
	        this.ancestorIds = source["ancestorIds"];
	        this.ancestors = this.convertValues(source["ancestors"], engine.EditAncestor);
	        this.field = source["field"];
	        this.position = source["position"];
	        this.value = source["value"];
	        this.startLine = source["startLine"];
	        this.endLine = source["endLine"];
	        this.status = source["status"];
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
	export class FileEditState {
	    edits: EditSummary[];
	    liftedEdits?: EditSummary[];
	    workingCode: string;
	    renderDiagnostics?: string[];
	    diagnostics?: string[];
	    valid: boolean;

	    static createFrom(source: any = {}) {
	        return new FileEditState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.edits = this.convertValues(source["edits"], EditSummary);
	        this.liftedEdits = this.convertValues(source["liftedEdits"], EditSummary);
	        this.workingCode = source["workingCode"];
	        this.renderDiagnostics = source["renderDiagnostics"];
	        this.diagnostics = source["diagnostics"];
	        this.valid = source["valid"];
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
	export class RevisionOption {
	    kind: string;
	    ref: string;
	    hash: string;
	    shortHash: string;
	    date: string;
	    author: string;
	    subject: string;

	    static createFrom(source: any = {}) {
	        return new RevisionOption(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.ref = source["ref"];
	        this.hash = source["hash"];
	        this.shortHash = source["shortHash"];
	        this.date = source["date"];
	        this.author = source["author"];
	        this.subject = source["subject"];
	    }
	}
	export class RevisionContext {
	    branch: string;
	    currentCommit: string;
	    options: RevisionOption[];

	    static createFrom(source: any = {}) {
	        return new RevisionContext(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.currentCommit = source["currentCommit"];
	        this.options = this.convertValues(source["options"], RevisionOption);
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

export namespace workspace {

	export class Edge {
	    id: string;
	    fromTileId: string;
	    toTileId: string;
	    kind: string;
	    label: string;

	    static createFrom(source: any = {}) {
	        return new Edge(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fromTileId = source["fromTileId"];
	        this.toTileId = source["toTileId"];
	        this.kind = source["kind"];
	        this.label = source["label"];
	    }
	}
	export class EdgeRef {
	    edgeId: string;
	    relationship: string;
	    fromTileId: string;

	    static createFrom(source: any = {}) {
	        return new EdgeRef(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.edgeId = source["edgeId"];
	        this.relationship = source["relationship"];
	        this.fromTileId = source["fromTileId"];
	    }
	}
	export class PaneState {
	    overviewCollapsed: boolean;
	    textCollapsed: boolean;

	    static createFrom(source: any = {}) {
	        return new PaneState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overviewCollapsed = source["overviewCollapsed"];
	        this.textCollapsed = source["textCollapsed"];
	    }
	}
	export class TextView {
	    language?: string;
	    filename?: string;
	    content?: string;
	    sourceStartLine?: number;
	    occurrences?: explorer.Occurrence[];

	    static createFrom(source: any = {}) {
	        return new TextView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.filename = source["filename"];
	        this.content = source["content"];
	        this.sourceStartLine = source["sourceStartLine"];
	        this.occurrences = this.convertValues(source["occurrences"], explorer.Occurrence);
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
	export class Overview {
	    kind: string;
	    title: string;
	    subtitle?: string;
	    packages?: explorer.Package[];
	    files?: explorer.File[];
	    declarations?: explorer.Declaration[];
	    importPaths?: string[];
	    imports?: explorer.ImportSummary[];
	    references?: explorer.Reference[];
	    selectedDeclaration?: explorer.Declaration;

	    static createFrom(source: any = {}) {
	        return new Overview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	        this.packages = this.convertValues(source["packages"], explorer.Package);
	        this.files = this.convertValues(source["files"], explorer.File);
	        this.declarations = this.convertValues(source["declarations"], explorer.Declaration);
	        this.importPaths = source["importPaths"];
	        this.imports = this.convertValues(source["imports"], explorer.ImportSummary);
	        this.references = this.convertValues(source["references"], explorer.Reference);
	        this.selectedDeclaration = this.convertValues(source["selectedDeclaration"], explorer.Declaration);
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
	export class Target {
	    kind: string;
	    packagePath?: string;
	    packageName?: string;
	    filePath?: string;
	    declarationName?: string;
	    line?: number;
	    endLine?: number;

	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.packagePath = source["packagePath"];
	        this.packageName = source["packageName"];
	        this.filePath = source["filePath"];
	        this.declarationName = source["declarationName"];
	        this.line = source["line"];
	        this.endLine = source["endLine"];
	    }
	}
	export class Tile {
	    id: string;
	    column: number;
	    target: Target;
	    openedBy?: EdgeRef;
	    previouslyOpened?: boolean;
	    existingTileId?: string;
	    overview: Overview;
	    text: TextView;
	    panes: PaneState;
	    collapsed: boolean;
	    canGoBack: boolean;

	    static createFrom(source: any = {}) {
	        return new Tile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.column = source["column"];
	        this.target = this.convertValues(source["target"], Target);
	        this.openedBy = this.convertValues(source["openedBy"], EdgeRef);
	        this.previouslyOpened = source["previouslyOpened"];
	        this.existingTileId = source["existingTileId"];
	        this.overview = this.convertValues(source["overview"], Overview);
	        this.text = this.convertValues(source["text"], TextView);
	        this.panes = this.convertValues(source["panes"], PaneState);
	        this.collapsed = source["collapsed"];
	        this.canGoBack = source["canGoBack"];
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
	export class InquiryRow {
	    id: string;
	    title?: string;
	    tiles: Tile[];
	    edges?: Edge[];

	    static createFrom(source: any = {}) {
	        return new InquiryRow(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.tiles = this.convertValues(source["tiles"], Tile);
	        this.edges = this.convertValues(source["edges"], Edge);
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


	export class ProgramState {
	    path: string;
	    module?: string;
	    packages?: explorer.Package[];
	    localImports?: string[];

	    static createFrom(source: any = {}) {
	        return new ProgramState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.module = source["module"];
	        this.packages = this.convertValues(source["packages"], explorer.Package);
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
	export class Selection {
	    rowId?: string;
	    tileId?: string;

	    static createFrom(source: any = {}) {
	        return new Selection(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rowId = source["rowId"];
	        this.tileId = source["tileId"];
	    }
	}
	export class State {
	    revision: number;
	    program?: ProgramState;
	    rows: InquiryRow[];
	    active: Selection;

	    static createFrom(source: any = {}) {
	        return new State(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.revision = source["revision"];
	        this.program = this.convertValues(source["program"], ProgramState);
	        this.rows = this.convertValues(source["rows"], InquiryRow);
	        this.active = this.convertValues(source["active"], Selection);
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
