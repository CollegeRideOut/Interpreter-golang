import { useEffect, useRef, useState } from 'react';
import { createWailsEngine, type Declaration, type EditSummary, type File, type FileEditState, type HeadlessState, type ImportSummary, type InquiryEngine, type InquiryRow, type Occurrence, type Package, type Reference, type RevisionContext, type RevisionOption, type Tile } from './inquiryEngine';

type FocusTarget =
  | { kind: 'symbol'; symbolId: string; name: string }
  | { kind: 'import'; path: string; name?: string };

type PackageLens = 'files' | 'api';
type FileLens = 'all' | 'exported' | 'internal';

type ComparisonFile = { packageDirectory: string; packageName: string; path: string; edits: EditSummary[]; declaration?: Declaration };
type EditInquiry = { id: string; title: string; columns: ComparisonFile[][] };

type ComparisonTreeNode = {
  id: string;
  nodeKind: string;
  field?: string;
  value?: string;
  startLine?: number;
  endLine?: number;
  edit?: EditSummary;
  children: ComparisonTreeNode[];
  parent?: ComparisonTreeNode;
};

export function App({ providedEngine }: { providedEngine?: InquiryEngine } = {}) {
  const [directory, setDirectory] = useState('../TestProgram');
  const [state, setState] = useState<HeadlessState | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [revisionContext, setRevisionContext] = useState<RevisionContext | null>(null);
  const [currentRevision, setCurrentRevision] = useState('working-tree');
  const [compareRevision, setCompareRevision] = useState('');
  const [editMap, setEditMap] = useState<Record<string, FileEditState>>({});
  const [comparisonActive, setComparisonActive] = useState(false);
  const [completeFileByTile, setCompleteFileByTile] = useState<Record<string, boolean>>({});
  const [engine] = useState<InquiryEngine>(() => providedEngine ?? createWailsEngine());
  const [focus, setFocus] = useState<FocusTarget | null>(null);
  const textRefs = useRef<Record<string, HTMLPreElement | null>>({});
  const [lenses, setLenses] = useState<Record<string, PackageLens | FileLens>>({});

  useEffect(() => {
    engine.getCurrentState().then(setState).catch(() => undefined);
  }, [engine]);

  useEffect(() => {
    if (!state?.program?.path) return;
    engine.getRevisionContext(state.program.path).then((context) => {
      setRevisionContext(context);
      setCompareRevision((current) => current || context.currentCommit);
    }).catch(() => setRevisionContext(null));
  }, [engine, state?.program?.path]);

  useEffect(() => {
    if (!comparisonActive || !state?.program?.path || !compareRevision) return;
    const requests = state.rows.flatMap((row) => row.tiles).flatMap((tile) => {
      const packageDirectory = tile.target.packagePath ?? '';
      const packageName = tile.target.packageName ?? '';
      if (tile.target.kind === 'program') {
        return (tile.overview.packages ?? []).flatMap((pkg) => (pkg.files ?? []).map((file) => ({ packageDirectory: pkg.directory, packageName: pkg.name, filePath: file.path })));
      }
      if (tile.target.kind === 'package') {
        return (tile.overview.files ?? []).map((file) => ({ packageDirectory, packageName, filePath: file.path }));
      }
      return tile.target.filePath && (tile.target.kind === 'file' || tile.target.kind === 'declaration') ? [{ packageDirectory, packageName, filePath: tile.target.filePath }] : [];
    }).map(async ({ packageDirectory, packageName, filePath }) => {
      const key = `${packageDirectory}:${packageName}:${filePath}`;
      try {
        return [key, await engine.getFileEditState(state.program?.path ?? directory, currentRevision, compareRevision, packageDirectory, packageName, filePath)] as const;
      } catch (reason) {
        throw reason instanceof Error ? reason : new Error('Unable to load comparison edits.');
      }
    });
    Promise.all(requests).then((entries) => {
      setEditMap((current) => ({ ...current, ...Object.fromEntries(entries) }));
    }).catch((reason) => {
      setError(reason instanceof Error ? reason.message : 'Unable to load comparison edits.');
    });
  }, [compareRevision, comparisonActive, currentRevision, directory, engine, state]);

  useEffect(() => {
    if (!state || !focus) return;
    for (const row of state.rows) {
      for (const tile of row.tiles) {
        const pre = textRefs.current[tile.id];
        const match = pre?.querySelector<HTMLElement>('[data-focus-match="true"]');
        if (pre && match) {
          const top = Math.max(0, match.offsetTop - pre.clientHeight / 2);
          if (typeof pre.scrollTo === 'function') {
            pre.scrollTo({ top, behavior: 'smooth' });
          } else {
            pre.scrollTop = top;
          }
        }
      }
    }
  }, [state, focus]);

  async function update(command: () => Promise<HeadlessState>) {
    setLoading(true);
    setError('');
    try {
      setState(await command());
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to update the inquiry.');
    } finally {
      setLoading(false);
    }
  }

  function exploreProgram() {
    setCurrentRevision('working-tree');
    setComparisonActive(false);
    setEditMap({});
    update(() => engine.openProgram(directory));
  }

  function selectCurrentRevision(revision: string) {
    setCurrentRevision(revision);
    setComparisonActive(false);
    setEditMap({});
    if (state?.program?.path) update(() => engine.selectRevision(state.program.path, revision));
  }

  function generateEdits() {
    setError('');
    setEditMap({});
    setComparisonActive(true);
  }

  function startInquiry() {
    update(() => engine.startInquiry('New inquiry'));
  }

  function togglePane(rowID: string, tile: Tile, pane: 'overview' | 'text') {
    const collapsed = pane === 'overview' ? tile.panes.overviewCollapsed : tile.panes.textCollapsed;
    update(() => engine.setPane(rowID, tile.id, pane, !collapsed));
  }

  function toggleTile(rowID: string, tile: Tile) {
    update(() => engine.setTileCollapsed(rowID, tile.id, !tile.collapsed));
  }

  function openPackage(row: InquiryRow, tile: Tile, pkg: Package) {
    update(() => engine.navigatePackage(row.id, tile.id, pkg.directory, pkg.name));
  }

  function openPackageColumn(row: InquiryRow, tile: Tile, pkg: Package) {
    update(() => engine.openPackage(row.id, tile.id, pkg.directory, pkg.name));
  }

  function openPackageLeft(row: InquiryRow, tile: Tile, pkg: Package) {
    const target = tile.column > 0 ? row.tiles.find((candidate) => candidate.column === tile.column - 1) : undefined;
    if (target) update(() => engine.navigatePackage(row.id, target.id, pkg.directory, pkg.name));
  }

  function lensKey(tile: Tile): string {
    return `${tile.id}:${tile.target.kind}`;
  }

  function packageLens(tile: Tile): PackageLens {
    return (lenses[lensKey(tile)] as PackageLens | undefined) ?? 'files';
  }

  function fileLens(tile: Tile): FileLens {
    return (lenses[lensKey(tile)] as FileLens | undefined) ?? 'all';
  }

  function setLens(tile: Tile, lens: PackageLens | FileLens) {
    setLenses((current) => ({ ...current, [lensKey(tile)]: lens }));
  }

  function inspectFile(row: InquiryRow, tile: Tile, file: File) {
    setFocus(null);
    update(() => engine.inspectFile(row.id, tile.id, tile.target.packagePath ?? '', tile.target.packageName ?? '', file.path));
  }

  function inspectDeclaration(row: InquiryRow, tile: Tile, filePath: string, declaration: Declaration) {
    setFocus(declaration.symbolId ? { kind: 'symbol', symbolId: declaration.symbolId, name: declaration.name } : null);
    update(() => engine.inspectDeclaration(row.id, tile.id, tile.target.packagePath ?? '', tile.target.packageName ?? '', filePath, declaration.name, declaration.line));
  }

  function inspectReference(row: InquiryRow, tile: Tile, reference: Reference) {
    setFocus(reference.symbolId ? { kind: 'symbol', symbolId: reference.symbolId, name: reference.name } : null);
    update(() => engine.navigateFile(row.id, tile.id, reference.packageDirectory ?? '', reference.packageName ?? '', reference.filePath));
  }

  function openReferenceColumn(row: InquiryRow, tile: Tile, reference: Reference) {
    setFocus(reference.symbolId ? { kind: 'symbol', symbolId: reference.symbolId, name: reference.name } : null);
    update(() => engine.openFile(row.id, tile.id, reference.packageDirectory ?? '', reference.packageName ?? '', reference.filePath));
  }

  function openReferenceLeft(row: InquiryRow, tile: Tile, reference: Reference) {
    if (tile.column === 0) return;
    const leftTile = row.tiles.find((candidate) => candidate.column === tile.column - 1);
    if (!leftTile) return;
    setFocus(reference.symbolId ? { kind: 'symbol', symbolId: reference.symbolId, name: reference.name } : null);
    update(() => engine.navigateFile(row.id, leftTile.id, reference.packageDirectory ?? '', reference.packageName ?? '', reference.filePath));
  }

  function openImport(row: InquiryRow, tile: Tile, imported: ImportSummary) {
    setFocus({ kind: 'import', path: imported.path, name: imported.name });
    update(() => engine.navigatePackage(row.id, tile.id, imported.directory, imported.name));
  }

  function openImportColumn(row: InquiryRow, tile: Tile, imported: ImportSummary) {
    setFocus({ kind: 'import', path: imported.path, name: imported.name });
    update(() => engine.openPackage(row.id, tile.id, imported.directory, imported.name));
  }

  function openImportLeft(row: InquiryRow, tile: Tile, imported: ImportSummary) {
    const target = tile.column > 0 ? row.tiles.find((candidate) => candidate.column === tile.column - 1) : undefined;
    if (!target) return;
    setFocus({ kind: 'import', path: imported.path, name: imported.name });
    update(() => engine.navigatePackage(row.id, target.id, imported.directory, imported.name));
  }

  function openFile(row: InquiryRow, tile: Tile, file: File) {
    update(() => engine.navigateFile(row.id, tile.id, tile.target.packagePath ?? '', tile.target.packageName ?? '', file.path));
  }

  function openFileColumn(row: InquiryRow, tile: Tile, file: File) {
    update(() => engine.openFile(row.id, tile.id, tile.target.packagePath ?? '', tile.target.packageName ?? '', file.path));
  }

  function openFileLeft(row: InquiryRow, tile: Tile, file: File) {
    const target = tile.column > 0 ? row.tiles.find((candidate) => candidate.column === tile.column - 1) : undefined;
    if (target) update(() => engine.navigateFile(row.id, target.id, tile.target.packagePath ?? '', tile.target.packageName ?? '', file.path));
  }

  function openDeclaration(row: InquiryRow, tile: Tile, declaration: Declaration) {
    setFocus(declaration.symbolId ? { kind: 'symbol', symbolId: declaration.symbolId, name: declaration.name } : null);
    update(() => engine.navigateDeclaration(row.id, tile.id, tile.target.packagePath ?? '', tile.target.packageName ?? '', tile.target.filePath ?? '', declaration.name, declaration.line));
  }

  function openDeclarationColumn(row: InquiryRow, tile: Tile, filePath: string, declaration: Declaration) {
    setFocus(declaration.symbolId ? { kind: 'symbol', symbolId: declaration.symbolId, name: declaration.name } : null);
    update(() => engine.openDeclaration(row.id, tile.id, tile.target.packagePath ?? '', tile.target.packageName ?? '', filePath, declaration.name, declaration.line));
  }

  function openDeclarationLeft(row: InquiryRow, tile: Tile, filePath: string, declaration: Declaration) {
    const target = tile.column > 0 ? row.tiles.find((candidate) => candidate.column === tile.column - 1) : undefined;
    if (!target) return;
    setFocus(declaration.symbolId ? { kind: 'symbol', symbolId: declaration.symbolId, name: declaration.name } : null);
    update(() => engine.navigateDeclaration(row.id, target.id, tile.target.packagePath ?? '', tile.target.packageName ?? '', filePath, declaration.name, declaration.line));
  }

  function closeColumn(row: InquiryRow, tile: Tile) {
    update(() => engine.closeColumn(row.id, tile.id));
  }

  function closeTile(row: InquiryRow, tile: Tile) {
    update(() => engine.closeTile(row.id, tile.id));
  }

  function goBack(row: InquiryRow, tile: Tile) {
    setFocus(null);
    update(() => engine.back(row.id, tile.id));
  }

  function editsFor(tile: Tile): EditSummary[] {
    return editsForPath(tile.target.packagePath ?? '', tile.target.packageName ?? '', tile.target.filePath ?? '');
  }

  function editsForPath(packageDirectory: string, packageName: string, filePath: string): EditSummary[] {
    return editMap[`${packageDirectory}:${packageName}:${filePath}`]?.edits ?? [];
  }

  function workingCodeForPath(packageDirectory: string, packageName: string, filePath: string): FileEditState | undefined {
    return editMap[`${packageDirectory}:${packageName}:${filePath}`];
  }

  async function changeComparisonEdit(packageDirectory: string, packageName: string, file: string, edit: EditSummary) {
    if (!state?.program?.path) return;
    setLoading(true);
    setError('');
    try {
      const update = edit.status === 'applied' || edit.status === 'prepared'
        ? engine.removeFileEdit
        : engine.applyFileEdit;
      const fileState = await update(state.program.path, currentRevision, compareRevision, packageDirectory, packageName, file, edit.index);
      const key = `${packageDirectory}:${packageName}:${file}`;
      const nextMap = { ...editMap, [key]: fileState };
      setEditMap(nextMap);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to update the edit.');
    } finally {
      setLoading(false);
    }
  }

  const editInquiries = comparisonActive && state ? buildEditInquiries(state, editMap) : [];

  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark">ct</span><span>CONTUTS</span></div>
        <div className="sidebar-section">
          <p className="eyebrow">PROGRAM</p>
          <label htmlFor="directory">Directory</label>
          <input id="directory" value={directory} onChange={(event) => setDirectory(event.target.value)} placeholder="/path/to/program" />
          <button type="button" onClick={exploreProgram} disabled={loading || directory.trim() === ''}>{loading ? 'Opening...' : 'Explore program'}</button>
            {state && <p className="revision">Revision {state.revision} · {state.rows.length + editInquiries.length} {state.rows.length + editInquiries.length === 1 ? 'inquiry' : 'inquiries'}</p>}
          {error && <p className="error">{error}</p>}
        </div>
      </aside>

      <section className="workspace" aria-label="Contuts inquiry workspace">
        {!state && <div className="welcome"><h1>Contuts hello</h1><p>Open a program to begin an inquiry.</p></div>}
        {state && <>
            <div className="workspace-heading"><div><p className="eyebrow">INQUIRY WORKSPACE</p><h1>{state.program?.path}</h1>{focus && <p className="focus-status">Highlighting {focus.kind === 'symbol' ? focus.name : focus.path}<button type="button" className="clear-focus" onClick={() => setFocus(null)}>Clear</button></p>}<RevisionControls context={revisionContext} current={currentRevision} compare={compareRevision} canGenerate={currentRevision !== compareRevision && compareRevision !== ''} onCurrentChange={selectCurrentRevision} onCompareChange={(revision) => { setCompareRevision(revision); setComparisonActive(false); setEditMap({}); }} onGenerate={generateEdits} /></div><button type="button" className="new-inquiry" onClick={startInquiry} disabled={loading}>+ New inquiry</button></div>
           <div className="inquiry-list">
             {state.rows.map((row) => <InquiryRowView key={row.id} row={row} focus={focus} textRefs={textRefs} editsFor={editsFor} editsForPath={editsForPath} workingCodeForPath={workingCodeForPath} comparisonActive={comparisonActive} completeFileByTile={completeFileByTile} onCompleteFileChange={(tileID, value) => setCompleteFileByTile((current) => ({ ...current, [tileID]: value }))} onApplyEdit={changeComparisonEdit} onPackage={openPackage} onPackageColumn={openPackageColumn} onPackageLeft={openPackageLeft} onImport={openImport} onImportColumn={openImportColumn} onImportLeft={openImportLeft} onFile={openFile} onFileColumn={openFileColumn} onFileLeft={openFileLeft} onInspectFile={inspectFile} onInspectReference={inspectReference} onOpenReferenceLeft={openReferenceLeft} onOpenReferenceColumn={openReferenceColumn} onDeclaration={openDeclaration} onDeclarationColumn={openDeclarationColumn} onDeclarationLeft={openDeclarationLeft} onInspectDeclaration={inspectDeclaration} onLensChange={setLens} packageLens={packageLens} fileLens={fileLens} onTogglePane={togglePane} onToggleTile={toggleTile} onCloseColumn={closeColumn} onCloseTile={closeTile} onBack={goBack} />)}
             {editInquiries.map((inquiry) => <EditInquiryView key={inquiry.id} inquiry={inquiry} onApplyEdit={changeComparisonEdit} />)}
          </div>
          <button type="button" className="bottom-inquiry" onClick={startInquiry} disabled={loading}>+ Start another inquiry at the bottom</button>
        </>}
      </section>
    </main>
  );
}

export function buildEditInquiries(state: HeadlessState, editMap: Record<string, FileEditState>): EditInquiry[] {
  const files = new Map<string, File & { packageDirectory: string; packageName: string }>();
  for (const row of state.rows) {
    for (const tile of row.tiles) {
      for (const pkg of tile.overview.packages ?? []) {
        for (const file of pkg.files ?? []) files.set(`${pkg.directory}:${pkg.name}:${file.path}`, { ...file, packageDirectory: pkg.directory, packageName: pkg.name });
      }
      for (const file of tile.overview.files ?? []) {
        files.set(`${tile.target.packagePath ?? ''}:${tile.target.packageName ?? ''}:${file.path}`, { ...file, packageDirectory: tile.target.packagePath ?? '', packageName: tile.target.packageName ?? '' });
      }
    }
  }
  const groups = new Map<string, ComparisonFile>();
  for (const [key, editState] of Object.entries(editMap)) {
    if (editState.edits.length === 0) continue;
    const [packageDirectory, packageName, ...pathParts] = key.split(':');
    const path = pathParts.join(':');
    const file = files.get(key);
    for (const edit of editState.edits) {
      const declaration = declarationForLine(file?.declarations ?? [], edit.startLine);
      const declarationKey = declaration ? `${declaration.symbolId ?? declaration.name}:${declaration.line}` : 'file';
      const groupKey = `${key}:${declarationKey}`;
      const group = groups.get(groupKey) ?? { packageDirectory, packageName, path, edits: [], declaration };
      group.edits.push(edit);
      groups.set(groupKey, group);
    }
  }
  return Array.from(groups.values()).sort((left, right) => (left.edits[0]?.startLine ?? 0) - (right.edits[0]?.startLine ?? 0)).map((file, index) => ({
    id: `${file.packageDirectory}:${file.packageName}:${file.path}:${file.declaration?.line ?? 'file'}:${index}`,
    title: file.declaration ? `${file.declaration.kind} ${file.declaration.name}` : file.path,
    columns: editColumns(file),
  }));
}

function declarationForLine(declarations: Declaration[], line?: number): Declaration | undefined {
  if (line === undefined) return undefined;
  const matches = declarations.flatMap((declaration) => {
    const nested = declarationForLine(declaration.children ?? [], line);
    if (nested) return [nested];
    return declaration.line <= line && declaration.endLine >= line ? [declaration] : [];
  });
  return matches.sort((left, right) => (right.line - left.line) || (left.endLine - right.endLine))[0];
}

function editColumns(file: ComparisonFile): ComparisonFile[][] {
  const tree = buildComparisonTree(file.edits);
  if (tree.length <= 1 || tree[0].children.length === 0) return [[file]];
  return tree[0].children.map((child) => {
    const edits = file.edits.filter((edit) => edit.nodeGlobalId === child.id || edit.nodeId === child.id || edit.ancestorIds?.includes(child.id));
    return [{ ...file, edits }];
  }).filter((column) => column[0].edits.length > 0);
}

function EditInquiryView({ inquiry, onApplyEdit }: { inquiry: EditInquiry; onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void }) {
  return <section className="inquiry-row"><div className="row-heading"><span>{inquiry.title}</span><span className="row-count">{inquiry.columns.length} column{inquiry.columns.length === 1 ? '' : 's'}</span></div><div className="tile-strip">{inquiry.columns.map((column, index) => {
    const file = column[0];
    const tile = { target: { kind: 'file', packagePath: file.packageDirectory, packageName: file.packageName, filePath: file.path } } as Tile;
    return <article className="tile" key={`${inquiry.id}:${index}`}><div className="tile-meta"><span>Column {index + 1}</span></div><div className="tile-views"><section className="tile-view overview-view"><ComparisonTreeSection tile={tile} edits={file.edits} completeFile={false} onCompleteFileChange={() => undefined} onApplyEdit={onApplyEdit} /></section></div></article>;
  })}</div></section>;
}

function RevisionControls({ context, current, compare, canGenerate, onCurrentChange, onCompareChange, onGenerate }: { context: RevisionContext | null; current: string; compare: string; canGenerate: boolean; onCurrentChange: (value: string) => void; onCompareChange: (value: string) => void; onGenerate: () => void }) {
  if (!context) return null;
  const options: RevisionOption[] = [{ kind: 'working-tree', ref: 'Working tree', hash: '', shortHash: '', date: '', subject: 'Current files on disk' }, ...context.options];
  const label = (option: RevisionOption) => option.kind === 'working-tree' ? option.ref : `${option.ref} · ${option.shortHash} · ${formatRevisionDate(option.date)}${option.subject ? ` · ${option.subject}` : ''}`;
  return <section className="revision-context" aria-label="Revision comparison"><div><label htmlFor="current-revision">Current revision</label><select id="current-revision" value={current} onChange={(event) => onCurrentChange(event.target.value)}>{options.map((option) => <option key={`current:${option.kind}:${option.hash || option.ref}`} value={option.kind === 'working-tree' ? 'working-tree' : option.hash}>{label(option)}</option>)}</select></div><div><label htmlFor="compare-revision">Compare to</label><select id="compare-revision" value={compare} onChange={(event) => onCompareChange(event.target.value)}><option value="">Choose a branch or commit</option>{context.options.map((option) => <option key={`compare:${option.kind}:${option.hash}`} value={option.hash}>{label(option)}</option>)}</select></div><button type="button" className="compare-placeholder" disabled={!canGenerate} onClick={onGenerate}>Generate edits</button></section>;
}

function formatRevisionDate(value: string): string {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export function buildComparisonTree(edits: EditSummary[]): ComparisonTreeNode[] {
  const roots: ComparisonTreeNode[] = [];
  const nodes = new Map<string, ComparisonTreeNode>();

  const addChild = (parent: ComparisonTreeNode | undefined, node: ComparisonTreeNode) => {
    if (node.parent === parent && (parent ? parent.children.includes(node) : roots.includes(node))) return;
    if (node.parent) {
      node.parent.children = node.parent.children.filter((child) => child !== node);
    } else {
      const rootIndex = roots.indexOf(node);
      if (rootIndex >= 0) roots.splice(rootIndex, 1);
    }
    node.parent = parent;
    if (parent) parent.children.push(node);
    else roots.push(node);
  };

  for (const edit of edits) {
    let parent: ComparisonTreeNode | undefined;
    const ancestors = edit.ancestors ?? (edit.ancestorIds ?? (edit.parentId ? [edit.parentId] : [])).map((nodeId) => ({ nodeId, nodeKind: nodeId === edit.parentId ? (edit.parentKind ?? 'AST node') : 'AST node' }));
    for (const ancestor of ancestors) {
      const identity = ancestor.globalId ?? ancestor.nodeId;
      let node = nodes.get(identity);
      if (!node) {
        const ancestorEdit = edits.find((candidate) => (candidate.nodeGlobalId ?? candidate.nodeId) === identity || candidate.nodeId === ancestor.nodeId);
        node = { id: identity, nodeKind: ancestor.nodeKind, field: ancestor.field, value: ancestor.value, startLine: ancestor.startLine, endLine: ancestor.endLine, edit: ancestorEdit, children: [] };
        nodes.set(identity, node);
        addChild(parent, node);
      } else if (node.edit === undefined) {
        const ancestorEdit = edits.find((candidate) => (candidate.nodeGlobalId ?? candidate.nodeId) === identity || candidate.nodeId === ancestor.nodeId);
        if (ancestorEdit) {
          node.edit = ancestorEdit;
          node.nodeKind = ancestorEdit.nodeKind;
          node.field = ancestorEdit.field;
          node.value = ancestorEdit.value;
          node.startLine = ancestorEdit.startLine;
          node.endLine = ancestorEdit.endLine;
        }
      }
      addChild(parent, node);
      parent = node;
    }
    const identity = edit.nodeGlobalId ?? edit.nodeId;
    let node = nodes.get(identity);
    if (!node) {
      node = { id: identity, nodeKind: edit.nodeKind, field: edit.field, value: edit.value, startLine: edit.startLine, endLine: edit.endLine, edit, children: [] };
      nodes.set(identity, node);
      addChild(parent, node);
    } else {
      node.edit = edit;
      node.nodeKind = edit.nodeKind;
      node.field = edit.field;
      node.value = edit.value;
      node.startLine = edit.startLine;
      node.endLine = edit.endLine;
    }
    addChild(parent, node);
  }

  const sort = (items: ComparisonTreeNode[]) => {
    items.sort((left, right) => (left.startLine ?? Number.MAX_SAFE_INTEGER) - (right.startLine ?? Number.MAX_SAFE_INTEGER) || (left.edit?.position ?? 0) - (right.edit?.position ?? 0) || left.id.localeCompare(right.id));
    items.forEach((item) => sort(item.children));
  };
  sort(roots);
  return roots;
}

type RowProps = {
  row: InquiryRow;
  editsFor: (tile: Tile) => EditSummary[];
  editsForPath: (packageDirectory: string, packageName: string, filePath: string) => EditSummary[];
  workingCodeForPath: (packageDirectory: string, packageName: string, filePath: string) => FileEditState | undefined;
  comparisonActive: boolean;
  completeFileByTile: Record<string, boolean>;
  onCompleteFileChange: (tileID: string, value: boolean) => void;
  onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void;
  focus: FocusTarget | null;
  textRefs: React.MutableRefObject<Record<string, HTMLPreElement | null>>;
  onPackage: (row: InquiryRow, tile: Tile, pkg: Package) => void;
  onPackageColumn: (row: InquiryRow, tile: Tile, pkg: Package) => void;
  onPackageLeft: (row: InquiryRow, tile: Tile, pkg: Package) => void;
  onImport: (row: InquiryRow, tile: Tile, imported: ImportSummary) => void;
  onImportColumn: (row: InquiryRow, tile: Tile, imported: ImportSummary) => void;
  onImportLeft: (row: InquiryRow, tile: Tile, imported: ImportSummary) => void;
  onFile: (row: InquiryRow, tile: Tile, file: File) => void;
  onFileColumn: (row: InquiryRow, tile: Tile, file: File) => void;
  onFileLeft: (row: InquiryRow, tile: Tile, file: File) => void;
  onInspectFile: (row: InquiryRow, tile: Tile, file: File) => void;
  onInspectReference: (row: InquiryRow, tile: Tile, reference: Reference) => void;
  onOpenReferenceLeft: (row: InquiryRow, tile: Tile, reference: Reference) => void;
  onOpenReferenceColumn: (row: InquiryRow, tile: Tile, reference: Reference) => void;
  onDeclaration: (row: InquiryRow, tile: Tile, declaration: Declaration) => void;
  onDeclarationColumn: (row: InquiryRow, tile: Tile, filePath: string, declaration: Declaration) => void;
  onDeclarationLeft: (row: InquiryRow, tile: Tile, filePath: string, declaration: Declaration) => void;
  onInspectDeclaration: (row: InquiryRow, tile: Tile, filePath: string, declaration: Declaration) => void;
  onLensChange: (tile: Tile, lens: PackageLens | FileLens) => void;
  packageLens: (tile: Tile) => PackageLens;
  fileLens: (tile: Tile) => FileLens;
  onTogglePane: (rowID: string, tile: Tile, pane: 'overview' | 'text') => void;
  onToggleTile: (rowID: string, tile: Tile) => void;
  onCloseColumn: (row: InquiryRow, tile: Tile) => void;
  onCloseTile: (row: InquiryRow, tile: Tile) => void;
  onBack: (row: InquiryRow, tile: Tile) => void;
};

function InquiryRowView({ row, focus, editsFor, editsForPath, workingCodeForPath, comparisonActive, completeFileByTile, onCompleteFileChange, onApplyEdit, textRefs, onPackage, onPackageColumn, onPackageLeft, onImport, onImportColumn, onImportLeft, onFile, onFileColumn, onFileLeft, onInspectFile, onInspectReference, onOpenReferenceLeft, onOpenReferenceColumn, onDeclaration, onDeclarationColumn, onDeclarationLeft, onInspectDeclaration, onLensChange, packageLens, fileLens, onTogglePane, onToggleTile, onCloseColumn, onCloseTile, onBack }: RowProps) {
  return <section className="inquiry-row"><div className="row-heading"><span>{row.title ?? 'Inquiry'}</span><span className="row-count">{row.tiles.length} tile{row.tiles.length === 1 ? '' : 's'}</span></div><div className="tile-strip">
    {row.tiles.map((tile) => <article className="tile" key={tile.id}>
       <div className="tile-meta"><span>Column {tile.column + 1}</span><span className="tile-meta-actions">{tile.openedBy && <span className="relationship">{tile.openedBy.relationship}</span>}<button type="button" className="close-tile" onClick={() => onCloseTile(row, tile)} disabled={row.tiles.length === 1} aria-label={`Close column ${tile.column + 1}`}>Close</button><button type="button" className="close-tile" onClick={() => onBack(row, tile)} disabled={!tile.canGoBack}>Back</button><button type="button" className="close-tile" onClick={() => onToggleTile(row.id, tile)} aria-expanded={!tile.collapsed}>{tile.collapsed ? 'Open' : 'Collapse'}</button></span></div>
      {tile.previouslyOpened && <div className="cycle-note">Previously opened earlier in this inquiry.</div>}
      {!tile.collapsed && <div className="tile-views">
        <section className={`tile-view overview-view ${tile.panes.overviewCollapsed ? 'collapsed' : ''}`}>
          <button type="button" className="pane-heading" onClick={() => onTogglePane(row.id, tile, 'overview')}><span>Overview</span><span>{tile.panes.overviewCollapsed ? '+' : '−'}</span></button>
            {!tile.panes.overviewCollapsed && <Overview tile={tile} row={row} edits={editsFor(tile)} comparisonActive={comparisonActive} completeFile={completeFileByTile[tile.id] === true} onCompleteFileChange={(value) => onCompleteFileChange(tile.id, value)} onApplyEdit={onApplyEdit} editsForPath={editsForPath} packageLens={packageLens(tile)} fileLens={fileLens(tile)} onLensChange={onLensChange} onPackage={onPackage} onPackageColumn={onPackageColumn} onPackageLeft={onPackageLeft} onImport={onImport} onImportColumn={onImportColumn} onImportLeft={onImportLeft} onFile={onFile} onFileColumn={onFileColumn} onFileLeft={onFileLeft} onInspectDeclaration={onInspectDeclaration} onDeclaration={onDeclaration} onDeclarationColumn={onDeclarationColumn} onDeclarationLeft={onDeclarationLeft} onInspectReference={onInspectReference} onOpenReferenceLeft={onOpenReferenceLeft} onOpenReferenceColumn={onOpenReferenceColumn} />}
        </section>
        <section className={`tile-view text-view ${tile.panes.textCollapsed ? 'collapsed' : ''}`}>
          <button type="button" className="pane-heading" onClick={() => onTogglePane(row.id, tile, 'text')}><span>Text representation</span><span>{tile.panes.textCollapsed ? '+' : '−'}</span></button>
           {!tile.panes.textCollapsed && (tile.target.kind === 'package' ? <PackageSourcePane tile={tile} focus={focus} textRefs={textRefs} row={row} onInspectFile={onInspectFile} workingCodeForPath={workingCodeForPath} /> : <TextRepresentation tile={tile} focus={focus} textRefs={textRefs} editState={workingCodeForPath(tile.target.packagePath ?? '', tile.target.packageName ?? '', tile.target.filePath ?? '')} />)}
        </section>
      </div>}
    </article>)}
  </div></section>;
}

function TextRepresentation({ tile, focus, textRefs, editState }: { tile: Tile; focus: FocusTarget | null; textRefs: React.MutableRefObject<Record<string, HTMLPreElement | null>>; editState?: FileEditState }) {
  const content = editState?.workingCode || tile.text.content;
  const diagnostics = [...(editState?.diagnostics ?? []), ...(editState?.renderDiagnostics ?? [])];
  if (!content) {
    return <><AstStatus editState={editState} /><pre ref={(element) => { textRefs.current[tile.id] = element; }}><code>No text representation for this target.</code></pre></>;
  }
  const lines = content.split('\n');
  const sourceStartLine = tile.text.sourceStartLine ?? 1;
  return <><AstStatus editState={editState} /><pre ref={(element) => { textRefs.current[tile.id] = element; }}><code>{lines.map((line, index) => {
    const sourceLine = sourceStartLine + index;
    const matches = occurrencesForLine(tile.text.occurrences ?? [], focus, sourceLine);
    return <span className={matches.length > 0 ? 'source-line focused' : 'source-line'} data-focus-match={matches.length > 0 ? 'true' : undefined} key={`${tile.id}:${index}`}>{highlightSource(line, matches, sourceLine)}{index < lines.length - 1 ? '\n' : ''}</span>;
  })}</code></pre>{diagnostics.length > 0 && <div className="render-diagnostics">{diagnostics.map((diagnostic, index) => <div key={`${tile.id}:diagnostic:${index}`}>{diagnostic}</div>)}</div>}</>;
}

function AstStatus({ editState }: { editState?: FileEditState }) {
  if (!editState || editState.valid) return null;
  return <div className="ast-status" role="status"><strong>AST is in an invalid state</strong><span>The best-effort renderer is showing the current tree.</span></div>;
}

function occurrencesForLine(occurrences: Occurrence[], focus: FocusTarget | null, line: number): Occurrence[] {
  if (!focus || focus.kind !== 'symbol') return [];
  return occurrences.filter((occurrence) => occurrence.symbolId === focus.symbolId && occurrence.startLine <= line && occurrence.endLine >= line);
}

function highlightSource(line: string, occurrences: Occurrence[], sourceLine: number): React.ReactNode {
  if (occurrences.length === 0) return line;
  const ranges = occurrences.map((occurrence) => ({
    start: occurrence.startLine === sourceLine ? occurrence.startColumn - 1 : 0,
    end: occurrence.endLine === sourceLine ? occurrence.endColumn - 1 : line.length,
  })).sort((left, right) => left.start - right.start);
  const parts: React.ReactNode[] = [];
  let cursor = 0;
  for (const range of ranges) {
    const start = Math.max(cursor, Math.min(line.length, range.start));
    const end = Math.max(start, Math.min(line.length, range.end));
    if (start > cursor) parts.push(line.slice(cursor, start));
    if (end > start) parts.push(<mark key={`${start}:${end}`}>{line.slice(start, end)}</mark>);
    cursor = end;
  }
  if (cursor < line.length) parts.push(line.slice(cursor));
  return parts;
}

function Overview({ tile, row, edits, comparisonActive, completeFile, onCompleteFileChange, onApplyEdit, editsForPath, packageLens, fileLens, onLensChange, onPackage, onPackageColumn, onPackageLeft, onImport, onImportColumn, onImportLeft, onFile, onFileColumn, onFileLeft, onInspectDeclaration, onDeclaration, onDeclarationColumn, onDeclarationLeft, onInspectReference, onOpenReferenceLeft, onOpenReferenceColumn }: Omit<RowProps, 'onTogglePane' | 'onToggleTile' | 'onCloseColumn' | 'onCloseTile' | 'onBack' | 'focus' | 'textRefs' | 'onInspectFile' | 'onInspectReference' | 'onOpenReferenceLeft' | 'onOpenReferenceColumn' | 'editsFor' | 'editsForPath' | 'packageLens' | 'fileLens' | 'comparisonActive' | 'completeFileByTile' | 'onCompleteFileChange' | 'comparisonRows' | 'onApplyEdit'> & { tile: Tile; edits: EditSummary[]; comparisonActive: boolean; completeFile: boolean; onCompleteFileChange: (value: boolean) => void; onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void; editsForPath: (packageDirectory: string, packageName: string, filePath: string) => EditSummary[]; packageLens: PackageLens; fileLens: FileLens; onInspectReference: (row: InquiryRow, tile: Tile, reference: Reference) => void; onOpenReferenceLeft: (row: InquiryRow, tile: Tile, reference: Reference) => void; onOpenReferenceColumn: (row: InquiryRow, tile: Tile, reference: Reference) => void }) {
  const overview = tile.overview;
  const [showReferences, setShowReferences] = useState(false);
  const declarationsFor = (file: File): Declaration[] => file.declarations ?? [];
  const visibleDeclaration = (declaration: Declaration): boolean => {
    if (fileLens === 'all') return true;
    if (fileLens === 'exported') return declaration.exported || (declaration.children ?? []).some((child) => child.exported);
    return !declaration.exported || (declaration.children ?? []).some((child) => !child.exported);
  };
  const declarationRows = (file: File): Array<{ declaration: Declaration; child: boolean }> => declarationsFor(file).flatMap((declaration) => {
    if (!visibleDeclaration(declaration)) return [];
    return [{ declaration, child: false }, ...(declaration.children ?? []).filter((child) => fileLens === 'all' || (fileLens === 'exported' ? child.exported : !child.exported)).map((child) => ({ declaration: child, child: true }))];
  });
  const packageApiRows = (file: File): Array<{ declaration: Declaration; child: boolean }> => declarationsFor(file).flatMap((declaration) => {
    if (!declaration.exported) return [];
    return [{ declaration, child: false }, ...(declaration.children ?? []).filter((child) => child.exported).map((child) => ({ declaration: child, child: true }))];
  });
  const editDeclaration = overview.selectedDeclaration ?? (tile.target.kind === 'declaration' ? overview.declarations?.[0] : undefined);
  return <div className="overview-content"><h2>{overview.title}</h2>{overview.subtitle && <p className="subtitle">{overview.subtitle}</p>}
     {tile.target.kind === 'package' && <LensControls value={packageLens} options={[['files', 'Files'], ['api', 'Exported API']]} onChange={(value) => onLensChange(tile, value as PackageLens)} />}
     {tile.target.kind === 'file' && <LensControls value={fileLens} options={[['all', 'All'], ['exported', 'Exported'], ['internal', 'Internal']]} onChange={(value) => onLensChange(tile, value as FileLens)} />}
     {tile.target.kind === 'package' && <ChangedFilesSection tile={tile} row={row} editsForPath={editsForPath} onFile={onFile} onFileLeft={onFileLeft} onFileColumn={onFileColumn} />}
     {overview.packages?.map((pkg) => <TargetButton key={`${pkg.directory}:${pkg.name}`} label={`package ${pkg.name}`} detail={`${pkg.directory || 'root package'} · ${pkg.fileCount} files`} canOpenLeft={tile.column > 0} onOpen={() => onPackage(row, tile, pkg)} onOpenLeft={() => onPackageLeft(row, tile, pkg)} onOpenColumn={() => onPackageColumn(row, tile, pkg)} />)}
     {tile.target.kind === 'package' && packageLens === 'files' && overview.files?.map((file) => <TargetButton key={file.path} label={file.name} detail={file.path} canOpenLeft={tile.column > 0} onOpen={() => onFile(row, tile, file)} onOpenLeft={() => onFileLeft(row, tile, file)} onOpenColumn={() => onFileColumn(row, tile, file)} />)}
      {tile.target.kind === 'package' && packageLens === 'api' && overview.files?.flatMap((file) => packageApiRows(file).map(({ declaration, child }) => <TargetButton key={`${file.path}:${declaration.symbolId ?? declaration.name}:${declaration.line}`} label={`${child ? '↳ ' : ''}${declaration.kind} ${declaration.name}`} detail={`${file.name} · ${declarationSignature(declaration)} · line ${declaration.line}`} canOpenLeft={tile.column > 0} onOpen={() => onInspectDeclaration(row, tile, file.path, declaration)} onOpenLeft={() => onDeclarationLeft(row, tile, file.path, declaration)} onOpenColumn={() => onDeclarationColumn(row, tile, file.path, declaration)} />))}
    {overview.importPaths && overview.importPaths.length > 0 && <section className="overview-section"><h3>Imports</h3>{overview.importPaths.map((path) => {
      const imported = overview.imports?.find((candidate) => candidate.path === path);
      if (!imported) {
        return <div className="import-row" key={path}><code>{path}</code></div>;
      }
       return <TargetButton key={path} label={imported.name} detail={path} canOpenLeft={tile.column > 0} onOpen={() => onImport(row, tile, imported)} onOpenLeft={() => onImportLeft(row, tile, imported)} onOpenColumn={() => onImportColumn(row, tile, imported)} />;
    })}</section>}
     {tile.target.kind === 'file' && overview.declarations?.flatMap((declaration) => {
      if (!visibleDeclaration(declaration)) return [];
      return [{ declaration, child: false }, ...(declaration.children ?? []).filter((child) => fileLens === 'all' || (fileLens === 'exported' ? child.exported : !child.exported)).map((child) => ({ declaration: child, child: true }))];
      }).map(({ declaration, child }) => <TargetButton key={`${declaration.symbolId ?? declaration.name}:${declaration.line}`} label={`${child ? '↳ ' : ''}${declaration.kind} ${declaration.name}`} detail={`${declarationSignature(declaration)} · line ${declaration.line}`} canOpenLeft={tile.column > 0} onOpen={() => onDeclaration(row, tile, declaration)} onOpenLeft={() => onDeclarationLeft(row, tile, tile.target.filePath ?? '', declaration)} onOpenColumn={() => onDeclarationColumn(row, tile, tile.target.filePath ?? '', declaration)} />)}
       {(tile.target.kind === 'declaration' || overview.selectedDeclaration) && (overview.references?.length ?? 0) > 0 && <section className="overview-section references-section"><h3>{overview.selectedDeclaration ? `${overview.selectedDeclaration.kind} ${overview.selectedDeclaration.name}` : 'References'}</h3><button type="button" className="section-toggle" onClick={() => setShowReferences((visible) => !visible)}>{showReferences ? 'Hide references' : `Find references (${overview.references?.length})`}</button>{showReferences && overview.references?.map((reference) => <ReferenceButton key={`${reference.filePath}:${reference.referenceLine ?? reference.line}`} reference={reference} canOpenLeft={tile.column > 0} onOpen={() => onInspectReference(row, tile, reference)} onOpenLeft={() => onOpenReferenceLeft(row, tile, reference)} onOpenColumn={() => onOpenReferenceColumn(row, tile, reference)} />)}</section>}
       {comparisonActive && (tile.target.kind === 'file' || tile.target.kind === 'declaration') ? <ComparisonTreeSection tile={tile} edits={edits} completeFile={completeFile} onCompleteFileChange={onCompleteFileChange} onApplyEdit={onApplyEdit} /> : editsForDeclaration(edits, editDeclaration).length > 0 && <EditSection edits={editsForDeclaration(edits, editDeclaration)} />}
    </div>;
}

function ComparisonTreeSection({ tile, edits, completeFile, onCompleteFileChange, onApplyEdit }: { tile: Tile; edits: EditSummary[]; completeFile: boolean; onCompleteFileChange: (value: boolean) => void; onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void }) {
  const tree = buildComparisonTree(edits);
  return <section className="overview-section edit-section comparison-results"><div className="comparison-tree-heading"><div><h3>Compared edits ({edits.length})</h3><p>Complete canonical edit tree. Structural replacements apply together.</p></div><span className="comparison-scope">Complete file</span></div>{tree.length === 0 ? <p className="empty-note">No edits in this file.</p> : <div className="comparison-tree">{tree.map((node) => <ComparisonTreeNodeView key={node.id} node={node} depth={0} packageDirectory={tile.target.packagePath ?? ''} packageName={tile.target.packageName ?? ''} file={tile.target.filePath ?? ''} onApplyEdit={onApplyEdit} />)}</div>}</section>;
}

function ComparisonTreeNodeView({ node, depth, packageDirectory, packageName, file, onApplyEdit }: { node: ComparisonTreeNode; depth: number; packageDirectory: string; packageName: string; file: string; onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void }) {
  const [expanded, setExpanded] = useState(true);
  const edit = node.edit;
  return <div className="comparison-tree-node"><div className={`comparison-tree-item ${edit ? 'comparison-tree-edit' : 'comparison-tree-context'}`} style={{ marginLeft: `${depth * 1.1}rem` }}><button type="button" className="tree-toggle" onClick={() => setExpanded((value) => !value)} disabled={node.children.length === 0} aria-label={node.children.length === 0 ? 'Leaf node' : expanded ? 'Collapse node' : 'Expand node'}>{node.children.length === 0 ? '·' : expanded ? '−' : '+'}</button><span className="tree-node-label">{edit && <span className="tree-edit-kind">{edit.kind}</span>}<strong>{node.nodeKind.replace('*ast.', '')}</strong>{node.field && <span className="tree-field">{node.field}</span>}{node.startLine !== undefined && <span className="tree-location">line {node.startLine}{node.endLine !== undefined && node.endLine !== node.startLine ? `-${node.endLine}` : ''}</span>}{node.value && <code>{node.value}</code>}</span>{edit && <button type="button" className="edit-action" onClick={() => onApplyEdit(packageDirectory, packageName, file, edit)} disabled={edit.status === 'prepared'}>{edit.status === 'applied' || edit.status === 'prepared' ? 'Remove' : edit.kind === 'DELETE' ? 'Apply replacement' : 'Apply'}</button>}</div>{expanded && node.children.map((child) => <ComparisonTreeNodeView key={child.id} node={child} depth={depth + 1} packageDirectory={packageDirectory} packageName={packageName} file={file} onApplyEdit={onApplyEdit} />)}</div>;
}

function ChangedFilesSection({ tile, row, editsForPath, onFile, onFileLeft, onFileColumn }: { tile: Tile; row: InquiryRow; editsForPath: (packageDirectory: string, packageName: string, filePath: string) => EditSummary[]; onFile: (row: InquiryRow, tile: Tile, file: File) => void; onFileLeft: (row: InquiryRow, tile: Tile, file: File) => void; onFileColumn: (row: InquiryRow, tile: Tile, file: File) => void }) {
  const changedFiles = (tile.overview.files ?? []).filter((file) => editsForPath(tile.target.packagePath ?? '', tile.target.packageName ?? '', file.path).length > 0);
  if (changedFiles.length === 0) return null;
  return <section className="overview-section"><h3>Changed files ({changedFiles.length})</h3>{changedFiles.map((file) => { const count = editsForPath(tile.target.packagePath ?? '', tile.target.packageName ?? '', file.path).length; return <TargetButton key={file.path} label={file.name} detail={`${count} AST edit${count === 1 ? '' : 's'} · ${file.path}`} canOpenLeft={tile.column > 0} onOpen={() => onFile(row, tile, file)} onOpenLeft={() => onFileLeft(row, tile, file)} onOpenColumn={() => onFileColumn(row, tile, file)} />; })}</section>;
}

function editsForDeclaration(edits: EditSummary[], declaration?: Declaration): EditSummary[] {
  if (!declaration) return edits;
  return edits.filter((edit) => !edit.startLine || !edit.endLine || (edit.startLine <= declaration.endLine && edit.endLine >= declaration.line));
}

function EditSection({ edits }: { edits: EditSummary[] }) {
  return <section className="overview-section edit-section"><h3>Edits ({edits.length})</h3>{edits.map((edit) => <div className="edit-row" key={`${edit.index}:${edit.nodeId}`}><strong>{edit.kind} {edit.nodeKind.replace('*ast.', '')}</strong><small>{edit.field ? `${edit.field} · ` : ''}line {edit.startLine || '?'}{edit.endLine && edit.endLine !== edit.startLine ? `-${edit.endLine}` : ''}{edit.value ? ` · ${edit.value}` : ''}</small></div>)}</section>;
}

function LensControls({ value, options, onChange }: { value: string; options: Array<[string, string]>; onChange: (value: string) => void }) {
  return <div className="lens-controls" role="group" aria-label="Overview lens">{options.map(([key, label]) => <button type="button" key={key} className={value === key ? 'lens-button active' : 'lens-button'} onClick={() => onChange(key)}>{label}</button>)}</div>;
}

function declarationSignature(declaration: Declaration): string {
  if (declaration.kind !== 'function' && declaration.kind !== 'method') return declaration.type ?? '';
  const parameters = (declaration.parameters ?? []).map((parameter) => parameter.name ? `${parameter.name} ${parameter.type}` : parameter.type).join(', ');
  const results = (declaration.results ?? []).map((result) => result.type).join(', ');
  return `(${parameters})${results ? ` -> ${results}` : ''}`;
}

function PackageSourcePane({ tile, focus, textRefs, row, onInspectFile, workingCodeForPath }: { tile: Tile; focus: FocusTarget | null; textRefs: React.MutableRefObject<Record<string, HTMLPreElement | null>>; row: InquiryRow; onInspectFile: (row: InquiryRow, tile: Tile, file: File) => void; workingCodeForPath: (packageDirectory: string, packageName: string, filePath: string) => FileEditState | undefined }) {
  const files = tile.overview.files ?? [];
  return <div className="package-source-pane"><nav className="file-rail" aria-label="Package files"><div className="file-rail-title">FILES</div>{files.map((file) => <button type="button" className={tile.text.filename === file.path ? 'file-rail-item selected' : 'file-rail-item'} key={file.path} onClick={() => onInspectFile(row, tile, file)}><strong>{file.name}</strong><small>{file.path}</small></button>)}</nav><div className="package-source"><TextRepresentation tile={tile} focus={focus} textRefs={textRefs} editState={workingCodeForPath(tile.target.packagePath ?? '', tile.target.packageName ?? '', tile.text.filename ?? '')} /></div></div>;
}

function TargetButton({ label, detail, canOpenLeft, onOpen, onOpenLeft, onOpenColumn }: { label: string; detail: string; canOpenLeft: boolean; onOpen: () => void; onOpenLeft: () => void; onOpenColumn: () => void }) {
  return <div className="target-choice"><button type="button" className="target-button" onClick={onOpen}><strong>{label}</strong><small>{detail}</small></button>{canOpenLeft && <button type="button" className="target-column-button" onClick={onOpenLeft}>Open left</button>}<button type="button" className="target-column-button" onClick={onOpenColumn}>+ column</button></div>;
}

function ReferenceButton({ reference, canOpenLeft, onOpen, onOpenLeft, onOpenColumn }: { reference: Reference; canOpenLeft: boolean; onOpen: () => void; onOpenLeft: () => void; onOpenColumn: () => void }) {
  return <div className="target-choice reference-choice"><button type="button" className="target-button" aria-label="Open reference in current tile" onClick={onOpen}><strong>{reference.filePath}</strong><small>{reference.kind} {reference.declaration} · line {reference.referenceLine ?? reference.line}</small></button>{canOpenLeft && <button type="button" className="target-column-button" aria-label="Open reference in left column" onClick={onOpenLeft}>Open left</button>}<button type="button" className="target-column-button" aria-label="Open reference in new column" onClick={onOpenColumn}>+ column</button></div>;
}
