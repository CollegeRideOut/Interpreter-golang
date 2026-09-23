export type Package = {
  name: string;
  directory: string;
  fileCount: number;
  files?: File[];
};

export type File = {
  name: string;
  path: string;
  declarations?: Declaration[];
  imports?: ImportSummary[];
};

export type Declaration = {
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
  children?: Declaration[];
};

export type Parameter = {
  name?: string;
  type: string;
};

export type ImportSummary = {
  path: string;
  name: string;
  directory: string;
  files: File[];
  declarations: Declaration[];
};

export type Reference = {
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
};

export type Tile = {
  id: string;
  column: number;
  target: {
    kind: string;
    packagePath?: string;
    packageName?: string;
    filePath?: string;
    declarationName?: string;
    line?: number;
  };
  openedBy?: {
    relationship: string;
  };
  previouslyOpened?: boolean;
  existingTileId?: string;
  canGoBack?: boolean;
  overview: {
    kind: string;
    title: string;
    subtitle?: string;
    packages?: Package[];
    files?: File[];
    declarations?: Declaration[];
    importPaths?: string[];
    imports?: ImportSummary[];
    references?: Reference[];
    selectedDeclaration?: Declaration;
  };
  text: {
    language?: string;
    filename?: string;
    content?: string;
    sourceStartLine?: number;
    occurrences?: Occurrence[];
  };
  panes: {
    overviewCollapsed: boolean;
    textCollapsed: boolean;
  };
  collapsed?: boolean;
};

export type Occurrence = {
  symbolId: string;
  name: string;
  startLine: number;
  startColumn: number;
  endLine: number;
  endColumn: number;
};

export type InquiryRow = {
  id: string;
  title?: string;
  tiles: Tile[];
};

export type HeadlessState = {
  revision: number;
  program?: {
    path: string;
  };
  rows: InquiryRow[];
  active: {
    rowId?: string;
    tileId?: string;
  };
};

export type RevisionOption = {
  kind: string;
  ref: string;
  hash: string;
  shortHash: string;
  date: string;
  author?: string;
  subject: string;
};

export type RevisionContext = {
  branch: string;
  currentCommit: string;
  options: RevisionOption[];
};

export type EditSummary = {
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
  ancestors?: EditAncestor[];
  field?: string;
  position: number;
  value?: string;
  startLine?: number;
  endLine?: number;
  status?: 'unapplied' | 'prepared' | 'applied' | 'removed';
};

export type EditAncestor = {
  nodeId: string;
  globalId?: string;
  nodeKind: string;
  field?: string;
  value?: string;
  startLine?: number;
  endLine?: number;
};

export type FileEditState = {
  edits: EditSummary[];
  liftedEdits?: EditSummary[];
  workingCode: string;
  renderDiagnostics?: string[];
  diagnostics?: string[];
  valid: boolean;
};

export interface InquiryEngine {
  getCurrentState(): Promise<HeadlessState>;
  getRevisionContext(directory: string): Promise<RevisionContext>;
  openProgram(directory: string): Promise<HeadlessState>;
  selectRevision(directory: string, revision: string): Promise<HeadlessState>;
  getFileEdits(directory: string, currentRevision: string, compareRevision: string, packageDirectory: string, packageName: string, filePath: string): Promise<EditSummary[]>;
  getFileEditState(directory: string, currentRevision: string, compareRevision: string, packageDirectory: string, packageName: string, filePath: string): Promise<FileEditState>;
  applyFileEdit(directory: string, currentRevision: string, compareRevision: string, packageDirectory: string, packageName: string, filePath: string, index: number): Promise<FileEditState>;
  removeFileEdit(directory: string, currentRevision: string, compareRevision: string, packageDirectory: string, packageName: string, filePath: string, index: number): Promise<FileEditState>;
  startInquiry(title: string): Promise<HeadlessState>;
  openPackage(rowID: string, tileID: string, packageDirectory: string, packageName: string): Promise<HeadlessState>;
  navigatePackage(rowID: string, tileID: string, packageDirectory: string, packageName: string): Promise<HeadlessState>;
  back(rowID: string, tileID: string): Promise<HeadlessState>;
  inspectFile(rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string): Promise<HeadlessState>;
  inspectDeclaration(rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string, name: string, line: number): Promise<HeadlessState>;
  openFile(rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string): Promise<HeadlessState>;
  navigateFile(rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string): Promise<HeadlessState>;
  openDeclaration(rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string, name: string, line: number): Promise<HeadlessState>;
  navigateDeclaration(rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string, name: string, line: number): Promise<HeadlessState>;
  setPane(rowID: string, tileID: string, pane: string, collapsed: boolean): Promise<HeadlessState>;
  setTileCollapsed(rowID: string, tileID: string, collapsed: boolean): Promise<HeadlessState>;
  closeColumn(rowID: string, tileID: string): Promise<HeadlessState>;
  closeTile(rowID: string, tileID: string): Promise<HeadlessState>;
}

type WailsEngine = {
  OpenProgram: (directory: string) => Promise<unknown>;
  SelectRevision: (directory: string, revision: string) => Promise<HeadlessState>;
  GetFileEdits: (directory: string, currentRevision: string, compareRevision: string, packageDirectory: string, packageName: string, filePath: string) => Promise<EditSummary[]>;
  GetFileEditState: (directory: string, currentRevision: string, compareRevision: string, packageDirectory: string, packageName: string, filePath: string) => Promise<FileEditState>;
  ApplyFileEdit: (directory: string, currentRevision: string, compareRevision: string, packageDirectory: string, packageName: string, filePath: string, index: number) => Promise<FileEditState>;
  RemoveFileEdit: (directory: string, currentRevision: string, compareRevision: string, packageDirectory: string, packageName: string, filePath: string, index: number) => Promise<FileEditState>;
  GetCurrentState: () => Promise<HeadlessState>;
  GetRevisionContext: (directory: string) => Promise<RevisionContext>;
  StartInquiry: (title: string) => Promise<HeadlessState>;
  OpenInquiryPackage: (rowID: string, tileID: string, packageDirectory: string, packageName: string) => Promise<HeadlessState>;
  NavigateInquiryPackage: (rowID: string, tileID: string, packageDirectory: string, packageName: string) => Promise<HeadlessState>;
  BackInquiry: (rowID: string, tileID: string) => Promise<HeadlessState>;
  InspectInquiryFile: (rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string) => Promise<HeadlessState>;
  InspectInquiryDeclaration: (rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string, name: string, line: number) => Promise<HeadlessState>;
  OpenInquiryFile: (rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string) => Promise<HeadlessState>;
  NavigateInquiryFile: (rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string) => Promise<HeadlessState>;
  OpenInquiryDeclaration: (rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string, name: string, line: number) => Promise<HeadlessState>;
  NavigateInquiryDeclaration: (rowID: string, tileID: string, packageDirectory: string, packageName: string, filePath: string, name: string, line: number) => Promise<HeadlessState>;
  SetInquiryPane: (rowID: string, tileID: string, pane: string, collapsed: boolean) => Promise<HeadlessState>;
  SetInquiryTileCollapsed: (rowID: string, tileID: string, collapsed: boolean) => Promise<HeadlessState>;
  CloseInquiryColumn: (rowID: string, tileID: string) => Promise<HeadlessState>;
  CloseInquiryTile: (rowID: string, tileID: string) => Promise<HeadlessState>;
};

declare global {
  interface Window {
    go?: {
      main?: {
        App?: WailsEngine;
      };
    };
  }
}

export function createWailsEngine(): InquiryEngine {
  const app = () => {
    const value = window.go?.main?.App;
    if (!value) {
      throw new Error('The Wails bridge is not available.');
    }
    return value;
  };
  return {
    getCurrentState: () => Promise.resolve().then(() => app().GetCurrentState()),
    getRevisionContext: (directory) => app().GetRevisionContext(directory),
    openProgram: async (directory) => {
      await app().OpenProgram(directory);
      return app().GetCurrentState();
    },
    selectRevision: (directory, revision) => app().SelectRevision(directory, revision),
    getFileEdits: (directory, currentRevision, compareRevision, packageDirectory, packageName, filePath) => app().GetFileEdits(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath),
    getFileEditState: (directory, currentRevision, compareRevision, packageDirectory, packageName, filePath) => app().GetFileEditState(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath),
    applyFileEdit: (directory, currentRevision, compareRevision, packageDirectory, packageName, filePath, index) => app().ApplyFileEdit(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath, index),
    removeFileEdit: (directory, currentRevision, compareRevision, packageDirectory, packageName, filePath, index) => app().RemoveFileEdit(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath, index),
    startInquiry: (title) => app().StartInquiry(title),
    openPackage: (rowID, tileID, packageDirectory, packageName) => app().OpenInquiryPackage(rowID, tileID, packageDirectory, packageName),
    navigatePackage: (rowID, tileID, packageDirectory, packageName) => app().NavigateInquiryPackage(rowID, tileID, packageDirectory, packageName),
    back: (rowID, tileID) => app().BackInquiry(rowID, tileID),
    inspectFile: (rowID, tileID, packageDirectory, packageName, filePath) => app().InspectInquiryFile(rowID, tileID, packageDirectory, packageName, filePath),
    inspectDeclaration: (rowID, tileID, packageDirectory, packageName, filePath, name, line) => app().InspectInquiryDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name, line),
    openFile: (rowID, tileID, packageDirectory, packageName, filePath) => app().OpenInquiryFile(rowID, tileID, packageDirectory, packageName, filePath),
    navigateFile: (rowID, tileID, packageDirectory, packageName, filePath) => app().NavigateInquiryFile(rowID, tileID, packageDirectory, packageName, filePath),
    openDeclaration: (rowID, tileID, packageDirectory, packageName, filePath, name, line) => app().OpenInquiryDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name, line),
    navigateDeclaration: (rowID, tileID, packageDirectory, packageName, filePath, name, line) => app().NavigateInquiryDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name, line),
    setPane: (rowID, tileID, pane, collapsed) => app().SetInquiryPane(rowID, tileID, pane, collapsed),
    setTileCollapsed: (rowID, tileID, collapsed) => app().SetInquiryTileCollapsed(rowID, tileID, collapsed),
    closeColumn: (rowID, tileID) => app().CloseInquiryColumn(rowID, tileID),
    closeTile: (rowID, tileID) => app().CloseInquiryTile(rowID, tileID),
  };
}
