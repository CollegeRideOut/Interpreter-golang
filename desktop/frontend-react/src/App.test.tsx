import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { App, buildComparisonTree, buildEditInquiries } from './App';
import type { EditSummary, FileEditState, HeadlessState, InquiryEngine } from './inquiryEngine';

function fakeEngine(initialState?: HeadlessState, fileEdits: EditSummary[] = [{ index: 0, kind: 'UPDATE', nodeId: 'main.Decl', nodeKind: '*ast.FuncDecl', field: 'Body', position: 0, startLine: 1, endLine: 3 }], onOpenFile?: (path: string) => void, workingCode = ''): InquiryEngine {
  let state: HeadlessState = initialState ?? {
    revision: 1,
    program: { path: '/tmp/example' },
    rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'program' }, overview: { kind: 'program', title: 'example', packages: [{ name: 'main', directory: '', fileCount: 1, files: [] }] }, text: {}, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
    active: { rowId: 'row-1', tileId: 'tile-1' },
  };
  const result = async () => state;
  const fileEditState = (status?: EditSummary['status']): FileEditState => ({ edits: fileEdits.map((edit) => ({ ...edit, status })), workingCode, valid: true });
  return {
    getCurrentState: result,
    getRevisionContext: async () => ({ branch: 'main', currentCommit: 'abc1234', options: [{ kind: 'branch', ref: 'main', hash: 'abc1234', shortHash: 'abc1234', date: '2026-09-14T13:29:59-04:00', subject: 'latest change' }] }),
    openProgram: result,
    chooseDirectory: async () => '/tmp/selected-program',
    startOpenCode: async () => undefined,
    writeOpenCodeInput: async () => undefined,
    resizeOpenCode: async () => undefined,
    stopOpenCode: async () => undefined,
    selectRevision: result,
    getFileEdits: async () => fileEdits,
    getFileEditState: async () => fileEditState(),
    getComparisonFiles: async () => [],
    applyFileEdit: async () => fileEditState('applied'),
    applyFileEditSubtree: async () => fileEditState('applied'),
    removeFileEdit: async () => fileEditState('removed'),
    removeFileEditSubtree: async () => fileEditState('removed'),
    openComparisonFile: async (_directory, _currentRevision, _compareRevision, packageDirectory, packageName, filePath) => {
      const tileID = 'comparison-tile';
      state = { ...state, rows: [...state.rows, { id: 'comparison-row', title: filePath, tiles: [{ id: tileID, column: 0, target: { kind: 'file', packagePath: packageDirectory, packageName, filePath }, overview: { kind: 'file', title: filePath, declarations: [] }, text: {}, panes: { overviewCollapsed: false, textCollapsed: false } }] }], active: { rowId: 'comparison-row', tileId: tileID } };
      return state;
    },
    startInquiry: async () => {
      state = { ...state, revision: state.revision + 1, rows: [...state.rows, { id: 'row-2', title: 'New inquiry', tiles: state.rows[0].tiles.slice(0, 1) }] };
      return state;
    },
    openPackage: result,
    navigatePackage: result,
    back: result,
    inspectFile: result,
    inspectDeclaration: result,
    openFile: result,
    navigateFile: async (_rowID, _tileID, _packageDirectory, _packageName, filePath) => {
      onOpenFile?.(filePath);
      return result();
    },
    openDeclaration: async (_rowID, _tileID, _packageDirectory, _packageName, _filePath, name) => {
      return result();
    },
    openDeclarationLeft: result,
    navigateDeclaration: result,
    setPane: result,
    setTileCollapsed: result,
    closeColumn: result,
    closeTile: result,
  };
}

describe('App', () => {
  afterEach(cleanup);
  it('renders the directory explorer and hello workspace', () => {
    render(<App />);

    expect(screen.getByLabelText('Directory')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Choose folder' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Explore program' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Contuts hello' })).toBeInTheDocument();
  });

  it('puts the selected folder in the directory field', async () => {
    render(<App providedEngine={fakeEngine()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Choose folder' }));

    await waitFor(() => expect(screen.getByLabelText('Directory')).toHaveValue('/tmp/selected-program'));
  });

  it('renders headless rows and can create another inquiry', async () => {
    render(<App providedEngine={fakeEngine()} />);

    expect(await screen.findByText('example')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '+ New inquiry' }));

    await waitFor(() => expect(screen.getByText('New inquiry')).toBeInTheDocument());
  });

  it('highlights a selected declaration in the opened text', async () => {
    render(<App providedEngine={fakeEngine({
      revision: 1,
      program: { path: '/tmp/example' },
      rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'file', filePath: 'main.go' }, overview: { kind: 'file', title: 'main.go', declarations: [{ symbolId: 'example::Run', kind: 'function', name: 'Run', line: 1, endLine: 3, exported: true }] }, text: { content: 'func Run() {\n\tRun()\n}', occurrences: [{ symbolId: 'example::Run', name: 'Run', startLine: 1, startColumn: 6, endLine: 1, endColumn: 9 }, { symbolId: 'example::Run', name: 'Run', startLine: 2, startColumn: 2, endLine: 2, endColumn: 5 }] }, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
      active: { rowId: 'row-1', tileId: 'tile-1' },
    }, undefined, undefined, 'func Run() {\n\tRun()\n}')} />);

    fireEvent.click(await screen.findByRole('button', { name: /function Run/ }));

    await waitFor(() => expect(screen.getByText(/Highlighting Run/)).toBeInTheDocument());
    fireEvent.click(await screen.findByRole('button', { name: 'Generate edits' }));
    expect((await screen.findAllByText('Compared edits (1)')).length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: 'Open right' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open left' })).toBeInTheDocument();
    expect(document.querySelector('mark')?.textContent).toBe('Run');
  });

  it('shows package files and switches to the exported API lens', async () => {
    render(<App providedEngine={fakeEngine({
      revision: 1,
      program: { path: '/tmp/example' },
      rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'package', packageName: 'server', packagePath: 'example/server' }, overview: { kind: 'package', title: 'package server', files: [{ name: 'server.go', path: 'server/server.go', declarations: [{ symbolId: 'example/server::New', kind: 'function', name: 'New', line: 1, endLine: 3, exported: true }] }] }, text: {}, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
      active: { rowId: 'row-1', tileId: 'tile-1' },
    })} />);

    expect((await screen.findAllByRole('button', { name: /server\.go/ })).length).toBe(2);
    fireEvent.click(screen.getByRole('button', { name: 'Exported API' }));

    expect(screen.getByRole('button', { name: /function New/ })).toBeInTheDocument();
  });

  it('shows declaration references with current-tile and new-column actions', async () => {
    render(<App providedEngine={fakeEngine({
      revision: 1,
      program: { path: '/tmp/example' },
      rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'declaration', packageName: 'server', packagePath: 'example/server', filePath: 'server.go' }, overview: { kind: 'declaration', title: 'Run', references: [{ symbolId: 'example/server::Run', name: 'Run', kind: 'function', packagePath: 'example/server', filePath: 'other.go', declaration: 'Run', line: 3, referenceLine: 8 }] }, text: {}, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
      active: { rowId: 'row-1', tileId: 'tile-1' },
    })} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Find references (1)' }));
    expect(screen.getByRole('button', { name: 'Open reference in current tile' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open reference in new column' })).toBeInTheDocument();
  });

  it('opens AST edit inquiry rows from a revision comparison', async () => {
    render(<App providedEngine={fakeEngine({
      revision: 1,
      program: { path: '/tmp/example' },
      rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'file', packageName: 'main', filePath: 'main.go' }, overview: { kind: 'file', title: 'main.go' }, text: {}, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
      active: { rowId: 'row-1', tileId: 'tile-1' },
    })} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Generate edits' }));
    expect((await screen.findAllByText('Compared edits (1)')).length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('main.go').length).toBeGreaterThanOrEqual(1);
    fireEvent.click(screen.getAllByRole('button', { name: 'Apply' })[0]);
    expect((await screen.findAllByRole('button', { name: 'Remove' })).length).toBeGreaterThan(0);
  });

  it('shows comparison results from the program explorer tile', async () => {
    let openedFile = '';
    render(<App providedEngine={fakeEngine({
      revision: 1,
      program: { path: '/tmp/example' },
      rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'program' }, overview: { kind: 'program', title: 'example', packages: [{ name: 'main', directory: '', fileCount: 1, files: [{ name: 'main.go', path: 'main.go', declarations: [{ kind: 'function', name: 'main', line: 1, endLine: 3, exported: false }] }] }] }, text: {}, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
      active: { rowId: 'row-1', tileId: 'tile-1' },
    }, undefined, (path) => { openedFile = path; }, 'package main\n\nfunc main() {}\n')} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Generate edits' }));
    expect(await screen.findAllByText('main.go')).not.toHaveLength(0);
    expect(screen.getByText('Structural edits (1)')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'function main' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Format' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open in new inquiry' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getAllByText('package main').length).toBeGreaterThan(1));
    fireEvent.click(screen.getByRole('button', { name: 'Open in new inquiry' }));
    expect(await screen.findAllByText('main.go')).toHaveLength(2);
    expect(screen.queryByText('Structural edits (1)')).not.toBeInTheDocument();
    expect(openedFile).toBe('');
  });

  it('formats TSX with Prettier when the Format button is pressed', async () => {
    render(<App providedEngine={fakeEngine({
      revision: 1,
      program: { path: '/tmp/example' },
      rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'file', packageName: 'main', filePath: 'App.tsx' }, overview: { kind: 'file', title: 'App.tsx' }, text: { language: 'tsx', content: "function App(){return <main><h1>Hello</h1></main>}" }, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
      active: { rowId: 'row-1', tileId: 'tile-1' },
    })} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Format' }));
    await waitFor(() => expect(document.querySelector('pre')?.textContent).toContain('function App() {'));
    expect(document.querySelector('pre')?.textContent).toContain('  return (');
  });

  it('keeps readable blank lines between TSX statement sections', async () => {
    render(<App providedEngine={fakeEngine({
      revision: 1,
      program: { path: '/tmp/example' },
      rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'file', packageName: 'main', filePath: 'server.ts' }, overview: { kind: 'file', title: 'server.ts' }, text: { language: 'typescript', content: "const app = express();\nconst port = 3000;\napp.listen(port, () => console.log('ready'));" }, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
      active: { rowId: 'row-1', tileId: 'tile-1' },
    })} />);

    await waitFor(() => expect(document.querySelector('pre')?.textContent).toContain('const port = 3000;\n\napp.listen'));
  });

  it('renders cousin branches beneath one shared ancestor', async () => {
    const edits: EditSummary[] = [
      { index: 0, kind: 'UPDATE', nodeId: 'grandparent', nodeKind: '*ast.FuncDecl', ancestorIds: [], position: 0 },
      { index: 1, kind: 'UPDATE', nodeId: 'grandchild-1', nodeKind: '*ast.BasicLit', ancestorIds: ['grandparent', 'child-1'], position: 0 },
      { index: 2, kind: 'UPDATE', nodeId: 'grandchild-2', nodeKind: '*ast.BasicLit', ancestorIds: ['grandparent', 'child-2'], position: 1 },
    ];
    const tree = buildComparisonTree(edits);
    expect(tree).toHaveLength(1);
    expect(tree[0].id).toBe('grandparent');
    expect(tree[0].children.map((node) => node.children[0].id)).toEqual(['grandchild-1', 'grandchild-2']);
  });

  it('groups edits under the same unedited assignment context', () => {
    const tree = buildComparisonTree([
      { index: 0, kind: 'UPDATE', nodeId: 'lhs', nodeKind: '*ast.Ident', position: 0, ancestors: [{ nodeId: 'assignment', nodeKind: '*ast.AssignStmt', field: 'Lhs', startLine: 6 }] },
      { index: 1, kind: 'UPDATE', nodeId: 'rhs', nodeKind: '*ast.BasicLit', position: 1, ancestors: [{ nodeId: 'assignment', nodeKind: '*ast.AssignStmt', field: 'Rhs', startLine: 6 }] },
    ]);
    expect(tree).toHaveLength(1);
    expect(tree[0].id).toBe('assignment');
    expect(tree[0].children.map((node) => node.id)).toEqual(['lhs', 'rhs']);
  });

  it('scopes declaration trees to descendants and excludes file parents', () => {
    const declaration = { kind: 'function', name: 'main', line: 3, endLine: 8, exported: false };
    const tree = buildComparisonTree([
      { index: 0, kind: 'UPDATE', nodeId: 'file', nodeKind: '*ast.File', position: 0, startLine: 1, endLine: 10 },
      { index: 1, kind: 'UPDATE', nodeId: 'body', nodeKind: '*ast.BlockStmt', position: 1, startLine: 3, endLine: 8, ancestors: [{ nodeId: 'file', nodeKind: '*ast.File', startLine: 1, endLine: 10 }] },
    ].filter((edit) => edit.startLine !== 1), declaration);

    expect(tree.map((node) => node.id)).toEqual(['body']);
  });

  it('shows full-line actions for edits with child edits', async () => {
    render(<App providedEngine={fakeEngine({
      revision: 1,
      program: { path: '/tmp/example' },
      rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'file', packageName: 'main', filePath: 'main.go' }, overview: { kind: 'file', title: 'main.go', declarations: [] }, text: {}, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
      active: { rowId: 'row-1', tileId: 'tile-1' },
    }, [
      { index: 0, kind: 'UPDATE', nodeId: 'parent', nodeKind: '*ast.AssignStmt', position: 0 },
      { index: 1, kind: 'UPDATE', nodeId: 'child', nodeKind: '*ast.BasicLit', parentId: 'parent', position: 0 },
    ])} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Generate edits' }));
    expect((await screen.findAllByRole('button', { name: 'Apply full line' })).length).toBeGreaterThan(0);
    fireEvent.click((await screen.findAllByRole('button', { name: 'Apply full line' }))[0]);
    await waitFor(() => expect(screen.getAllByRole('button', { name: 'Remove full line' }).length).toBeGreaterThan(0));
  });

  it('renders declaration inquiries with declaration-only source and edits', async () => {
    render(<App providedEngine={fakeEngine({
      revision: 1,
      program: { path: '/tmp/example' },
      rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'declaration', packageName: 'main', packagePath: '', filePath: 'main.go', declarationName: 'main', line: 3 }, overview: { kind: 'declaration', title: 'main', subtitle: 'function', declarations: [{ kind: 'function', name: 'main', line: 3, endLine: 8, exported: false }], references: [] }, text: { content: 'package main\n\nimport "fmt"\n\nfunc main() {\n\twork()\n}', sourceStartLine: 1 }, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
      active: { rowId: 'row-1', tileId: 'tile-1' },
    }, [
      { index: 0, kind: 'INSERT', nodeId: 'import', nodeKind: '*ast.GenDecl', position: 0, startLine: 3, endLine: 3 },
      { index: 1, kind: 'UPDATE', nodeId: 'body', nodeKind: '*ast.BlockStmt', position: 1, startLine: 3, endLine: 8, ancestors: [{ nodeId: 'file', nodeKind: '*ast.File', startLine: 1, endLine: 10 }] },
    ], undefined, 'package main\n\nimport "fmt"\n\nfunc main() {\n\twork()\n}\n')} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Generate edits' }));
    expect(await screen.findByText('Compared edits (1)')).toBeInTheDocument();
    expect(screen.getByText('Edits inside function main.')).toBeInTheDocument();
    expect(screen.getByText('func main() {')).toBeInTheDocument();
    expect(screen.queryByText('package main')).not.toBeInTheDocument();
  });

  it('merges source and target paths for one assignment by global identity', () => {
    const tree = buildComparisonTree([
      {
        index: 0,
        kind: 'DELETE',
        nodeId: 'source-rhs',
        nodeKind: '*ast.Ident',
        parentGlobalId: 'assignment-global',
        field: 'Rhs',
        position: 0,
        ancestors: [{ nodeId: 'source-assignment', globalId: 'assignment-global', nodeKind: '*ast.AssignStmt', value: '=' }],
      },
      {
        index: 1,
        kind: 'UPDATE',
        nodeId: 'target-assignment',
        nodeGlobalId: 'assignment-global',
        nodeKind: '*ast.AssignStmt',
        value: ':=',
        position: 0,
        ancestors: [{ nodeId: 'function', globalId: 'function-global', nodeKind: '*ast.FuncDecl' }],
      },
    ]);
    expect(tree).toHaveLength(1);
    expect(tree[0].children).toHaveLength(1);
    expect(tree[0].children[0].edit?.nodeId).toBe('target-assignment');
    expect(tree[0].children[0].children[0].edit?.nodeId).toBe('source-rhs');
  });

  it('separates edits belonging to sibling declarations into inquiries', () => {
    const state: HeadlessState = {
      revision: 1,
      program: { path: '/tmp/example' },
      rows: [{ id: 'row-1', title: 'Program inquiry', tiles: [{ id: 'tile-1', column: 0, target: { kind: 'program' }, overview: { kind: 'program', title: 'example', packages: [{ name: 'main', directory: '', fileCount: 1, files: [{ name: 'main.go', path: 'main.go', declarations: [{ kind: 'function', name: 'one', line: 1, endLine: 3, exported: false }, { kind: 'function', name: 'two', line: 5, endLine: 7, exported: false }] }] }] }, text: {}, panes: { overviewCollapsed: false, textCollapsed: false } }] }],
      active: { rowId: 'row-1', tileId: 'tile-1' },
    };
    const inquiries = buildEditInquiries(state, { ':main:main.go': { edits: [{ index: 0, kind: 'UPDATE', nodeId: 'one', nodeKind: '*ast.FuncDecl', position: 0, startLine: 2, endLine: 2 }, { index: 1, kind: 'UPDATE', nodeId: 'two', nodeKind: '*ast.FuncDecl', position: 0, startLine: 6, endLine: 6 }], workingCode: '', valid: true } });
    expect(inquiries.map((inquiry) => inquiry.title)).toEqual(['function one', 'function two']);
  });
});
