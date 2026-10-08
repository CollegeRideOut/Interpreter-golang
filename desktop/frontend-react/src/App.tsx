import { useEffect, useRef, useState } from 'react';
import * as prettier from 'prettier/standalone';
import * as prettierBabel from 'prettier/plugins/babel';
import * as prettierEstree from 'prettier/plugins/estree';
import * as prettierTypescript from 'prettier/plugins/typescript';
import { FitAddon } from '@xterm/addon-fit';
import { Terminal } from '@xterm/xterm';
import { EventsOn } from '../wailsjs/runtime/runtime';
import '@xterm/xterm/css/xterm.css';
import { createWailsEngine, type ComparisonFile as ComparisonFileRecord, type Declaration, type EditSummary, type File, type FileEditState, type HeadlessState, type ImportSummary, type InquiryEngine, type InquiryRow, type Occurrence, type Package, type ProposalWorkspace, type Reference, type RevisionContext, type RevisionOption, type Tile } from './inquiryEngine';

type FocusTarget =
  | { kind: 'symbol'; symbolId: string; name: string }
  | { kind: 'import'; path: string; name?: string }
  | { kind: 'line'; path: string; startLine: number; endLine: number };

type PackageLens = 'files' | 'api' | 'internal';
type FileLens = 'all' | 'exported' | 'internal';

type ComparisonFile = { packageDirectory: string; packageName: string; path: string; edits: EditSummary[]; declaration?: Declaration };
type EditInquiry = { id: string; title: string; columns: ComparisonFile[][]; declaration?: Declaration };
const LAST_DIRECTORY_STORAGE_KEY = 'contuts.lastDirectory';

function rememberedDirectory(): string {
  try {
    return window.localStorage.getItem(LAST_DIRECTORY_STORAGE_KEY) || '../TestProgram';
  } catch {
    return '../TestProgram';
  }
}

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
  const [directory, setDirectory] = useState(rememberedDirectory);
  const [state, setState] = useState<HeadlessState | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [revisionContext, setRevisionContext] = useState<RevisionContext | null>(null);
  const [currentRevision, setCurrentRevision] = useState('working-tree');
  const [compareRevision, setCompareRevision] = useState('');
  const [compareRevisions, setCompareRevisions] = useState<string[]>([]);
  const [workspaceRevision, setWorkspaceRevision] = useState('working-tree');
  const [editMap, setEditMap] = useState<Record<string, FileEditState>>({});
  const [projectedFiles, setProjectedFiles] = useState<Record<string, string>>({});
  const [comparisonActive, setComparisonActive] = useState(false);
  const [completeFileByTile, setCompleteFileByTile] = useState<Record<string, boolean>>({});
  const [engine] = useState<InquiryEngine>(() => providedEngine ?? createWailsEngine());
  const [focus, setFocus] = useState<FocusTarget | null>(null);
  const textRefs = useRef<Record<string, HTMLPreElement | null>>({});
  const comparisonOpenKey = useRef<string | null>(null);
  const applyQueues = useRef<Record<string, Promise<void>>>({});
  const [lenses, setLenses] = useState<Record<string, PackageLens | FileLens>>({});
  const [terminalOpen, setTerminalOpen] = useState(false);
  const [terminalStarted, setTerminalStarted] = useState(false);
  const [openedComparisonFiles, setOpenedComparisonFiles] = useState<Record<string, boolean>>({});
  const [formatRequests, setFormatRequests] = useState<Record<string, number>>({});
  const [proposalWorkspace, setProposalWorkspace] = useState<ProposalWorkspace | null>(null);
  const [proposalSource, setProposalSource] = useState('');
  const [proposalSources, setProposalSources] = useState<string[]>([]);
  const [proposalMode, setProposalMode] = useState(false);
  const [activeReviewBranchId, setActiveReviewBranchId] = useState('');
  const [proposalFiles, setProposalFiles] = useState<ComparisonFileRecord[]>([]);
  const [proposalEditMap, setProposalEditMap] = useState<Record<string, Record<string, FileEditState>>>({});
  const [humanEditMap, setHumanEditMap] = useState<Record<string, FileEditState>>({});
  const [appliedProposalEdits, setAppliedProposalEdits] = useState<Record<string, boolean>>({});
  const activeReviewEditMap = proposalMode
    ? proposalEditMap[activeReviewBranchId] ?? {}
    : displayEditMap();
  const activeExplorerEditMap = proposalMode
    ? Object.fromEntries(Object.entries(activeReviewEditMap).map(([key, state]) => [key, { ...state, edits: humanEdits(state).map((edit) => ({ ...edit, proposalApplied: appliedProposalEdits[`${activeReviewBranchId}:${key}:${edit.index}`] === true })) }]))
    : activeReviewEditMap;
  const comparisonPreview = comparisonActive
    ? Object.values(proposalMode ? humanEditMap : activeReviewEditMap).map((editState) => proposalMode ? { ...editState, workingAuthoritative: true } : editState).find((editState) => editState.workingCode.trim() !== '')
      ?? Object.values(proposalMode ? humanEditMap : activeReviewEditMap).map((editState) => proposalMode ? { ...editState, workingAuthoritative: true } : editState)[0]
    : undefined;
  const proposalPreview = comparisonActive && proposalMode
    ? Object.values(activeReviewEditMap).map((editState) => ({ ...editState, workingAuthoritative: true })).find((editState) => editState.workingCode.trim() !== '')
      ?? Object.values(activeReviewEditMap).map((editState) => ({ ...editState, workingAuthoritative: true }))[0]
    : undefined;

  function comparisonPair() {
    return `${currentRevision}:${compareRevision}`;
  }

  function isProjected(key: string) {
    return projectedFiles[key] === comparisonPair();
  }

  function displayEditState(key: string, editState: FileEditState): FileEditState {
    return editState;
  }

  function humanEdits(editState: FileEditState): EditSummary[] {
    return editState.liftedEdits ?? editState.edits;
  }

  function displayEditMap() {
    return Object.fromEntries(Object.entries(editMap).map(([key, value]) => [key, displayEditState(key, value)]));
  }

  useEffect(() => {
    engine.getCurrentState().then(setState).catch(() => undefined);
  }, [engine]);

  useEffect(() => {
    if (!state?.program?.path) return;
    engine.getRevisionContext(state.program.path).then((context) => {
      setRevisionContext(context);
      setCompareRevision((current) => current || context.currentCommit);
      setCompareRevisions((current) => current.length > 0 ? current : context.currentCommit ? [context.currentCommit] : []);
    }).catch(() => setRevisionContext(null));
  }, [engine, state?.program?.path]);

  useEffect(() => {
    if (!state?.program?.path) return;
    engine.getProposalBranches(state.program.path, currentRevision, '').then(setProposalWorkspace).catch(() => undefined);
  }, [currentRevision, engine, state?.program?.path]);

  useEffect(() => {
    if (!comparisonActive || proposalMode || !state?.program?.path || !compareRevision) return;
    const loadComparison = async () => {
      const knownFiles = filesFromState(state);
      const revisionFiles = await engine.getComparisonFiles(state.program.path, currentRevision, compareRevision).catch(() => []);
      const proposals = await engine.getProposalBranches(state.program.path, currentRevision, compareRevision);
      setProposalWorkspace(proposals);
      setProposalSource((current) => current || compareRevision);
      const files = new Map<string, ComparisonFileRecord>();
      for (const file of [...knownFiles, ...revisionFiles]) {
        files.set(`${file.packageDirectory}:${file.packageName}:${file.file.path}`, file);
      }
      const requests = Array.from(files.values()).map(async ({ packageDirectory, packageName, file }) => {
        const key = `${packageDirectory}:${packageName}:${file.path}`;
        try {
          return [key, await engine.getFileEditState(state.program?.path ?? directory, currentRevision, compareRevision, packageDirectory, packageName, file.path)] as const;
        } catch {
          return null;
        }
      });
      const entries = await Promise.all(requests);
      const nextEditMap = Object.fromEntries(entries.filter((entry): entry is readonly [string, FileEditState] => entry !== null));
      setEditMap(nextEditMap);
      const openKey = `${currentRevision}:${compareRevision}`;
      comparisonOpenKey.current = openKey;
    };
    loadComparison().catch((reason) => {
      setError(reason instanceof Error ? reason.message : 'Unable to load comparison edits.');
    });
  }, [compareRevision, comparisonActive, currentRevision, directory, engine, proposalMode, proposalWorkspace?.activeBranchId, state]);

  useEffect(() => {
    if (!proposalMode || !state?.program?.path || !proposalWorkspace) return;
    const selected = proposalWorkspace.branches.filter((branch) => proposalWorkspace.selectedProposalIds?.includes(branch.id));
    if (selected.length === 0) return;
    const loadProposals = async () => {
      const files = new Map<string, ComparisonFileRecord>();
      for (const branch of selected) {
        const branchFiles = await engine.getComparisonFiles(state.program!.path, currentRevision, branch.sourceRevision ?? '').catch(() => []);
        for (const file of branchFiles) files.set(`${file.packageDirectory}:${file.packageName}:${file.file.path}`, file);
      }
      const allFiles = [...files.values()];
      const proposalEntries = await Promise.all(selected.map(async (branch) => {
        const entries = await Promise.all(allFiles.map(async ({ packageDirectory, packageName, file }) => {
          try {
            return [`${packageDirectory}:${packageName}:${file.path}`, await engine.getProposalFileEditState(state.program!.path, currentRevision, branch.id, packageDirectory, packageName, file.path)] as const;
          } catch {
            return null;
          }
        }));
        return [branch.id, Object.fromEntries(entries.filter((entry): entry is readonly [string, FileEditState] => entry !== null))] as const;
      }));
      const humanEntries = await Promise.all(allFiles.map(async ({ packageDirectory, packageName, file }) => {
        try {
          return [`${packageDirectory}:${packageName}:${file.path}`, await engine.getHumanFileEditState(state.program!.path, currentRevision, packageDirectory, packageName, file.path)] as const;
        } catch {
          return null;
        }
      }));
      setProposalFiles(allFiles);
      setProposalEditMap(Object.fromEntries(proposalEntries));
      setHumanEditMap(Object.fromEntries(humanEntries.filter((entry): entry is readonly [string, FileEditState] => entry !== null)));
    };
    loadProposals().catch((reason) => setError(reason instanceof Error ? reason.message : 'Unable to load proposal edits.'));
  }, [currentRevision, engine, proposalMode, proposalWorkspace, state]);

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

  async function reloadProgram(stopTerminal: boolean) {
    setCurrentRevision('working-tree');
    setWorkspaceRevision('working-tree');
    setComparisonActive(false);
    setEditMap({});
    setProposalWorkspace(null);
    setProposalSource('');
    setProposalSources([]);
    setCompareRevisions([]);
    setActiveReviewBranchId('');
    setHumanEditMap({});
    setAppliedProposalEdits({});
    setOpenedComparisonFiles({});
    comparisonOpenKey.current = null;
    if (stopTerminal && terminalStarted) {
      await engine.stopOpenCode().catch(() => undefined);
      setTerminalStarted(false);
      setTerminalOpen(false);
    }
    setLoading(true);
    setError('');
    try {
      const nextState = await engine.openProgram(directory);
      try {
        window.localStorage.setItem(LAST_DIRECTORY_STORAGE_KEY, directory);
      } catch {
        // Storage can be unavailable in restricted webviews; opening still works.
      }
      setState(nextState);
      const context = await engine.getRevisionContext(directory).catch(() => null);
      setRevisionContext(context);
      setCompareRevision(context?.currentCommit ?? '');
      setCompareRevisions(context?.currentCommit ? [context.currentCommit] : []);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to load the program.');
    } finally {
      setLoading(false);
    }
  }

  function exploreProgram() {
    void reloadProgram(true);
  }

  function refreshProgram() {
    void reloadProgram(false);
  }

  async function promoteHumanBuild() {
    if (!state?.program?.path) return;
    setLoading(true);
    setError('');
    try {
      const context = await engine.promoteHumanBuild(directory, currentRevision, 'contuts: commit Human build');
      const revision = context.currentCommit || 'working-tree';
      const nextState = await engine.selectRevision(directory, revision);
      setState(nextState);
      setRevisionContext(context);
      setCurrentRevision(revision);
      setCompareRevision('');
      setCompareRevisions([]);
      setWorkspaceRevision(revision);
      setComparisonActive(false);
      setProposalMode(false);
      setProposalWorkspace(null);
      setProposalEditMap({});
      setHumanEditMap({});
      setAppliedProposalEdits({});
      setEditMap({});
    } catch (reason) {
		setError(reason instanceof Error ? reason.message : 'Unable to commit the Human build.');
    } finally {
      setLoading(false);
    }
  }

  async function toggleTerminal() {
    if (terminalOpen) {
      setTerminalOpen(false);
      return;
    }
    setTerminalStarted(true);
    setTerminalOpen(true);
  }

  async function chooseDirectory() {
    setError('');
    try {
      const selected = await engine.chooseDirectory();
       if (selected) {
         setDirectory(selected);
         try {
           window.localStorage.setItem(LAST_DIRECTORY_STORAGE_KEY, selected);
         } catch {
           // Storage can be unavailable in restricted webviews; selection still works.
         }
       }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to choose a folder.');
    }
  }

  function selectCurrentRevision(revision: string) {
    setCurrentRevision(revision);
    setCompareRevision('');
    setCompareRevisions([]);
    setComparisonActive(false);
    setEditMap({});
    setProposalWorkspace(null);
    setProposalSource('');
    setProposalSources([]);
    setActiveReviewBranchId('');
    setHumanEditMap({});
    setAppliedProposalEdits({});
    setOpenedComparisonFiles({});
    setWorkspaceRevision(revision);
    if (state?.program?.path) update(() => engine.selectRevision(state.program.path, revision));
  }

  function selectCompareRevision(revision: string) {
    setCompareRevision(revision);
    setCompareRevisions(revision ? [revision] : []);
    setComparisonActive(false);
    setEditMap({});
    setProposalWorkspace(null);
    setProposalSource('');
    setProposalSources([]);
    setActiveReviewBranchId('');
    setHumanEditMap({});
    setAppliedProposalEdits({});
    setOpenedComparisonFiles({});
    comparisonOpenKey.current = null;
  }

  function generateEdits() {
    setError('');
    setOpenedComparisonFiles({});
    comparisonOpenKey.current = null;
    setComparisonActive(true);
  }

  function proposalNameForRevision(revision: string, fallback: string): string {
    return revisionContext?.options.find((option) => option.hash === revision)?.ref || fallback;
  }

  async function createProposal() {
    if (!state?.program?.path || !currentRevision) return;
    setLoading(true);
    setError('');
    try {
      const count = proposalWorkspace?.branches.filter((branch) => branch.id !== 'base').length ?? 0;
      const source = proposalSources[0] || compareRevisions[0] || proposalSource || compareRevision;
      if (!source || source === currentRevision) throw new Error('Choose a Git branch or commit for the proposal.');
      const next = await engine.createProposalBranch(state.program.path, currentRevision, proposalMode ? '' : compareRevision, source, `Proposal ${count + 1}`);
      setProposalWorkspace(next);
      setEditMap({});
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to create proposal.');
    } finally {
      setLoading(false);
    }
  }

  async function generateProposalEdits() {
    if (!state?.program?.path || !proposalWorkspace) return;
    setLoading(true);
    setError('');
    try {
      let next = proposalWorkspace;
      const existing = new Set(next.branches.filter((branch) => branch.kind === 'proposal').map((branch) => branch.sourceRevision));
      const sources = compareRevisions.filter((source) => source && source !== currentRevision && !existing.has(source));
      if (sources.length === 0) throw new Error('All selected revisions already have proposal targets. Select a different revision or review the existing proposal targets below.');
      for (const source of sources) {
        const count = next.branches.filter((branch) => branch.kind === 'proposal').length + 1;
        next = await engine.createProposalBranch(state.program.path, currentRevision, compareRevision, source, proposalNameForRevision(source, `Proposal ${count}`));
      }
      const selected = next.branches.filter((branch) => branch.kind === 'proposal' && (sources.includes(branch.sourceRevision ?? '') || next.selectedProposalIds?.includes(branch.id))).map((branch) => branch.id);
      if (selected.length === 0) throw new Error('Choose at least one branch or commit as a proposal source.');
      next = await engine.selectProposalBranches(state.program.path, currentRevision, selected);
      setProposalWorkspace(next);
      setProposalMode(true);
      setComparisonActive(true);
      setActiveReviewBranchId(selected[0] ?? '');
      setProposalSource('');
      setProposalSources([]);
      setEditMap({});
      setAppliedProposalEdits({});
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to generate proposal edits.');
    } finally {
      setLoading(false);
    }
  }

  async function reviewSelectedProposals() {
    if (!state?.program?.path || !proposalWorkspace) return;
    const selected = proposalWorkspace.selectedProposalIds ?? [];
    if (selected.length === 0) {
      setError('Create or select at least one proposal first.');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const next = await engine.selectProposalBranches(state.program.path, currentRevision, selected);
      setProposalWorkspace(next);
      setProposalMode(true);
      setComparisonActive(true);
      setActiveReviewBranchId(selected[0] ?? '');
      setProposalSource('');
      setEditMap({});
      setAppliedProposalEdits({});
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to review proposals.');
    } finally {
      setLoading(false);
    }
  }

  async function acceptProposalEdit(proposalID: string, file: ComparisonFileRecord, edit: EditSummary) {
    if (!state?.program?.path) return;
    setLoading(true);
    const key = `${file.packageDirectory}:${file.packageName}:${file.file.path}`;
    try {
      const human = await engine.copyProposalEdit(state.program.path, currentRevision, compareRevision, proposalID, file.packageDirectory, file.packageName, file.file.path, edit.index);
      setHumanEditMap((current) => ({ ...current, [key]: human }));
      setAppliedProposalEdits((current) => {
        const next = { ...current, [`${proposalID}:${key}:${edit.index}`]: true };
        const proposalState = activeReviewEditMap[key];
        if (!proposalState) return next;
        const covered = new Set([edit.nodeId]);
        for (const candidate of humanEdits(proposalState)) {
          if (candidate.index === edit.index || candidate.ancestorIds?.some((ancestor) => covered.has(ancestor))) {
            covered.add(candidate.nodeId);
            next[`${proposalID}:${key}:${candidate.index}`] = true;
          }
        }
        return next;
      });
      const refreshedWorkspace = await engine.getProposalBranches(state.program.path, currentRevision, compareRevision);
      setProposalWorkspace(refreshedWorkspace);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to accept proposal edit.');
    } finally {
      setLoading(false);
    }
  }

  async function selectProposal(branchID: string) {
    if (!state?.program?.path || !compareRevision) return;
    setLoading(true);
    setError('');
    try {
      const next = await engine.selectProposalBranch(state.program.path, currentRevision, compareRevision, branchID);
    setProposalWorkspace(next);
    setEditMap({});
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to select proposal.');
    } finally {
      setLoading(false);
    }
  }

  async function toggleProposalSelection(branchID: string) {
    if (!state?.program?.path || !proposalWorkspace) return;
    const current = new Set(proposalWorkspace.selectedProposalIds ?? []);
    if (current.has(branchID)) current.delete(branchID); else current.add(branchID);
    try {
      setProposalWorkspace(await engine.selectProposalBranches(state.program.path, currentRevision, [...current]));
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to select proposal.');
    }
  }

  function startInquiry() {
    update(() => engine.startInquiry('New inquiry'));
  }

  function openComparisonFileInquiry(file: ComparisonFile, edit?: EditSummary) {
    const key = `${file.packageDirectory}:${file.packageName}:${file.path}`;
    setOpenedComparisonFiles((current) => ({ ...current, [key]: true }));
    if (edit?.startLine) setFocus({ kind: 'line', path: file.path, startLine: edit.startLine, endLine: edit.endLine ?? edit.startLine });
    setLoading(true);
    setError('');
    engine.openComparisonFile(directory, currentRevision, compareRevision, file.packageDirectory, file.packageName, file.path)
      .then(setState)
      .catch((reason) => setError(reason instanceof Error ? reason.message : 'Unable to open the comparison file.'))
      .finally(() => setLoading(false));
  }

  function locateEdit(file: ComparisonFile, edit: EditSummary) {
    openComparisonFileInquiry(file, edit);
  }

  function openEditColumn(row: InquiryRow, tile: Tile, file: ComparisonFile) {
    openFileColumn(row, tile, file);
  }

  function requestFormat(tileID: string) {
    setFormatRequests((current) => ({ ...current, [tileID]: (current[tileID] ?? 0) + 1 }));
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
    setFocus(declaration.symbolId ? { kind: 'symbol', symbolId: declaration.symbolId, name: declaration.name } : null);
    const target = tile.column > 0 ? row.tiles.find((candidate) => candidate.column === tile.column - 1) : undefined;
    update(() => target
      ? engine.navigateDeclaration(row.id, target.id, tile.target.packagePath ?? '', tile.target.packageName ?? '', filePath, declaration.name, declaration.line)
      : engine.openDeclarationLeft(row.id, tile.id, tile.target.packagePath ?? '', tile.target.packageName ?? '', filePath, declaration.name, declaration.line));
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
    const key = `${packageDirectory}:${packageName}:${filePath}`;
    const editState = activeExplorerEditMap[key];
    return editState ? humanEdits(displayEditState(key, editState)) : [];
  }

  function workingCodeForPath(packageDirectory: string, packageName: string, filePath: string): FileEditState | undefined {
    const key = `${packageDirectory}:${packageName}:${filePath}`;
    const sourceMap = proposalMode ? humanEditMap : activeReviewEditMap;
    const editState = sourceMap[key];
    return editState ? displayEditState(key, proposalMode ? { ...editState, workingAuthoritative: true } : editState) : undefined;
  }

  function proposalCodeForPath(packageDirectory: string, packageName: string, filePath: string): FileEditState | undefined {
    if (!proposalMode) return undefined;
    const editState = activeReviewEditMap[`${packageDirectory}:${packageName}:${filePath}`];
    return editState ? { ...editState, workingAuthoritative: true } : undefined;
  }

  function applyVisibleEdit(packageDirectory: string, packageName: string, file: string, edit: EditSummary, fullLine = false) {
    const key = `${packageDirectory}:${packageName}:${file}`;
    const previous = applyQueues.current[key] ?? Promise.resolve();
    const next = previous.then(() => applyVisibleEditNow(packageDirectory, packageName, file, edit, fullLine));
    const queued = next.finally(() => {
      if (applyQueues.current[key] === queued) delete applyQueues.current[key];
    });
    applyQueues.current[key] = queued;
  }

  async function applyVisibleEditNow(packageDirectory: string, packageName: string, file: string, edit: EditSummary, fullLine = false) {
    if (proposalMode && activeReviewBranchId) {
      const proposalFile: ComparisonFileRecord = { packageDirectory, packageName, file: { name: file.split('/').pop() ?? file, path: file } };
      await acceptProposalEdit(activeReviewBranchId, proposalFile, edit);
      return;
    }
    if (!proposalMode) await changeComparisonEdit(packageDirectory, packageName, file, edit, fullLine);
  }

  async function changeComparisonEdit(packageDirectory: string, packageName: string, file: string, edit: EditSummary, fullLine = false) {
    if (!state?.program?.path) return;
    setLoading(true);
    setError('');
    try {
      const update = fullLine
        ? edit.status === 'applied' || edit.status === 'prepared' ? engine.removeFileEditSubtree : engine.applyFileEditSubtree
        : edit.status === 'applied' || edit.status === 'prepared' ? engine.removeFileEdit : engine.applyFileEdit;
      const fileState = await update(state.program.path, currentRevision, compareRevision, packageDirectory, packageName, file, edit.index);
      const key = `${packageDirectory}:${packageName}:${file}`;
      const nextMap = { ...editMap, [key]: fileState };
      setEditMap(nextMap);
      setProjectedFiles((current) => ({ ...current, [key]: comparisonPair() }));
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to update the edit.');
    } finally {
      setLoading(false);
    }
  }

  async function copyProposalEdit(packageDirectory: string, packageName: string, file: string, edit: EditSummary) {
    if (!state?.program?.path || !proposalWorkspace || proposalWorkspace.activeBranchId === 'base' || proposalWorkspace.activeBranchId === 'human') return;
    setLoading(true);
    setError('');
    try {
      await engine.copyProposalEdit(state.program.path, currentRevision, compareRevision, proposalWorkspace.activeBranchId, packageDirectory, packageName, file, edit.index);
      const next = await engine.selectProposalBranch(state.program.path, currentRevision, compareRevision, 'human');
       setProposalWorkspace(next);
       setEditMap({});
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to copy proposal edit.');
    } finally {
      setLoading(false);
    }
  }

  const canCopyProposal = !proposalMode && proposalWorkspace?.branches.find((branch) => branch.id === proposalWorkspace.activeBranchId)?.kind === 'proposal';
  const existingProposalSources = new Set(proposalWorkspace?.branches.filter((branch) => branch.kind === 'proposal').map((branch) => branch.sourceRevision).filter((source): source is string => Boolean(source)) ?? []);
  const availableProposalSources = compareRevisions.filter((source) => source && source !== currentRevision && !existingProposalSources.has(source));

  const editInquiries = comparisonActive && state
    ? buildEditInquiries(state, activeExplorerEditMap, proposalMode ? proposalFiles : []).filter((inquiry) => {
      const file = inquiry.columns[0]?.[0];
      return !file || !openedComparisonFiles[`${file.packageDirectory}:${file.packageName}:${file.path}`];
    })
    : [];

  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark">ct</span><span>CONTUTS</span></div>
        <div className="sidebar-section">
          <p className="eyebrow">PROGRAM</p>
          <label htmlFor="directory">Directory</label>
          <div className="directory-picker"><input id="directory" value={directory} onChange={(event) => setDirectory(event.target.value)} placeholder="/path/to/program" /><button type="button" className="choose-directory" onClick={chooseDirectory} disabled={loading}>Choose folder</button></div>
          <button type="button" onClick={exploreProgram} disabled={loading || directory.trim() === ''}>{loading ? 'Opening...' : 'Explore program'}</button>
          {state && <button type="button" className="refresh-program" onClick={refreshProgram} disabled={loading}>{loading ? 'Refreshing...' : 'Refresh project'}</button>}
            {state && <p className="revision">Revision {state.revision} · {state.rows.length} {state.rows.length === 1 ? 'inquiry' : 'inquiries'}</p>}
          {error && <p className="error">{error}</p>}
        </div>
      </aside>

      <section className="workspace" aria-label="Contuts inquiry workspace">
        {!state && <div className="welcome"><h1>Contuts hello</h1><p>Open a program to begin an inquiry.</p></div>}
        {state && <>
                <div className="workspace-heading"><div><p className="eyebrow">INQUIRY WORKSPACE</p><h1>{state.program?.path}</h1>{focus && <p className="focus-status">Highlighting {focus.kind === 'symbol' ? focus.name : focus.path}<button type="button" className="clear-focus" onClick={() => setFocus(null)}>Clear</button></p>}{proposalMode ? <div className="proposal-base"><strong>Explorer: Human build</strong><code>branch {revisionContext?.branch || 'detached'}</code><code>base {currentRevision}</code><button type="button" onClick={promoteHumanBuild} disabled={loading}>Commit Human build</button><button type="button" onClick={() => setProposalMode(false)}>Exit proposal review</button></div> : <RevisionControls context={revisionContext} current={currentRevision} compare={compareRevision} compareSelections={compareRevisions} workspace={workspaceRevision} canGenerate={currentRevision !== compareRevision && compareRevision !== ''} canGenerateProposals={availableProposalSources.length > 0} onCurrentChange={selectCurrentRevision} onCompareSelectionsChange={(revisions) => { setCompareRevisions(revisions); setCompareRevision(revisions[0] ?? ''); }} onGenerate={generateEdits} onGenerateProposals={generateProposalEdits} />}{proposalWorkspace && (proposalMode || proposalWorkspace.branches.some((branch) => branch.kind === 'proposal')) && <ProposalControls workspace={proposalWorkspace} disabled={loading} proposalMode={proposalMode} onToggle={toggleProposalSelection} onReview={reviewSelectedProposals} />}</div><div className="heading-actions"><button type="button" className="terminal-toggle" onClick={toggleTerminal}>{terminalOpen ? 'Close OpenCode' : 'Open OpenCode'}</button><button type="button" className="new-inquiry" onClick={startInquiry} disabled={loading}>+ New inquiry</button></div></div>
               {proposalMode && <div className="human-build-banner">The explorer shows your mutable Human build. Proposal targets stay immutable; apply only the structural edits you choose into Human build.</div>}
             <div className={proposalMode ? 'proposal-review-layout' : undefined}>
               {proposalMode && <ProposalTabs proposals={proposalWorkspace?.branches.filter((branch) => proposalWorkspace.selectedProposalIds?.includes(branch.id)) ?? []} activeBranchId={activeReviewBranchId} onSelect={setActiveReviewBranchId} />}
             <div className="inquiry-list">
                {state.rows.map((row) => <InquiryRowView key={row.id} row={row} editInquiries={editInquiries} comparisonPreview={comparisonPreview} proposalPreview={proposalPreview} formatRequests={formatRequests} onFormat={requestFormat} focus={focus} textRefs={textRefs} editsFor={editsFor} editsForPath={editsForPath} workingCodeForPath={workingCodeForPath} proposalCodeForPath={proposalCodeForPath} comparisonActive={comparisonActive} completeFileByTile={completeFileByTile} onCompleteFileChange={(tileID, value) => setCompleteFileByTile((current) => ({ ...current, [tileID]: value }))} onApplyEdit={applyVisibleEdit} onCopyEdit={copyProposalEdit} canCopyProposal={canCopyProposal} onOpenComparisonFile={openComparisonFileInquiry} onLocateEdit={locateEdit} onOpenEditColumn={openEditColumn} onPackage={openPackage} onPackageColumn={openPackageColumn} onPackageLeft={openPackageLeft} onImport={openImport} onImportColumn={openImportColumn} onImportLeft={openImportLeft} onFile={openFile} onFileColumn={openFileColumn} onFileLeft={openFileLeft} onInspectFile={inspectFile} onInspectReference={inspectReference} onOpenReferenceLeft={openReferenceLeft} onOpenReferenceColumn={openReferenceColumn} onDeclaration={openDeclaration} onDeclarationColumn={openDeclarationColumn} onDeclarationLeft={openDeclarationLeft} onInspectDeclaration={inspectDeclaration} onLensChange={setLens} packageLens={packageLens} fileLens={fileLens} onTogglePane={togglePane} onToggleTile={toggleTile} onCloseColumn={closeColumn} onCloseTile={closeTile} onBack={goBack} />)}
           </div></div>
          <button type="button" className="bottom-inquiry" onClick={startInquiry} disabled={loading}>+ Start another inquiry at the bottom</button>
        </>}
      </section>
      {terminalStarted && <OpenCodeTerminal engine={engine} directory={state.program?.path ?? directory} visible={terminalOpen} onClose={() => setTerminalOpen(false)} onStopped={() => { setTerminalStarted(false); setTerminalOpen(false); }} />}
    </main>
  );
}

function OpenCodeTerminal({ engine, directory, visible, onClose, onStopped }: { engine: InquiryEngine; directory: string; visible: boolean; onClose: () => void; onStopped: () => void }) {
  const terminalRef = useRef<HTMLDivElement | null>(null);
  const instanceRef = useRef<Terminal | null>(null);
  const [running, setRunning] = useState(true);

  useEffect(() => {
    const terminal = new Terminal({
      cursorBlink: true,
      fontFamily: '"JetBrains Mono", monospace',
      fontSize: 13,
      scrollback: 5000,
      theme: { background: '#0d1018', foreground: '#dfe2ec', cursor: '#b36aff' },
    });
    const fit = new FitAddon();
    terminal.loadAddon(fit);
    if (terminalRef.current) terminal.open(terminalRef.current);
    instanceRef.current = terminal;
    const resize = () => {
      fit.fit();
      engine.resizeOpenCode(terminal.cols, terminal.rows).catch(() => undefined);
    };
    const removeOutput = EventsOn('opencode:output', (output: string) => terminal.write(output));
    const removeStatus = EventsOn('opencode:status', (status: { running?: boolean; error?: string }) => {
      if (status.running === false) {
        setRunning(false);
        onStopped();
        if (status.error) terminal.write(`\r\n\x1b[31mOpenCode stopped: ${status.error}\x1b[0m\r\n`);
      }
    });
    const input = terminal.onData((data) => engine.writeOpenCodeInput(data).catch(() => undefined));
    const observer = new ResizeObserver(resize);
    if (terminalRef.current) observer.observe(terminalRef.current);
    resize();
    engine.startOpenCode(directory).catch((reason) => {
      terminal.write(`\r\n\x1b[31mUnable to start OpenCode: ${reason instanceof Error ? reason.message : String(reason)}\x1b[0m\r\n`);
      setRunning(false);
      onStopped();
    });
    return () => {
      observer.disconnect();
      input.dispose();
      removeOutput();
      removeStatus();
      terminal.dispose();
      instanceRef.current = null;
    };
  }, [directory, engine]);

  return <aside className={`opencode-terminal${visible ? '' : ' hidden'}`} aria-label="OpenCode terminal" aria-hidden={!visible}>
    <header className="terminal-header"><div><strong>OpenCode</strong><span>{directory}</span></div><div className="terminal-actions"><span className={running ? 'terminal-state running' : 'terminal-state'}>{running ? 'running' : 'stopped'}</span><button type="button" onClick={onClose}>Close</button></div></header>
    <div className="terminal-screen" ref={terminalRef} />
  </aside>;
}

export function buildEditInquiries(state: HeadlessState, editMap: Record<string, FileEditState>, comparisonFiles: ComparisonFile[] = []): EditInquiry[] {
  const files = new Map<string, File & { packageDirectory: string; packageName: string }>();
  for (const comparisonFile of comparisonFiles) {
    files.set(`${comparisonFile.packageDirectory}:${comparisonFile.packageName}:${comparisonFile.file.path}`, { ...comparisonFile.file, packageDirectory: comparisonFile.packageDirectory, packageName: comparisonFile.packageName });
  }
  for (const row of state.rows) {
    for (const tile of row.tiles) {
      for (const pkg of flattenPackages(tile.overview.packages ?? [])) {
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
    declaration: file.declaration,
  }));
}

function flattenPackages(packages: Package[]): Package[] {
  return packages.flatMap((pkg) => [pkg, ...flattenPackages(pkg.children ?? [])]);
}

function filesFromState(state: HeadlessState): ComparisonFile[] {
  const files = new Map<string, ComparisonFile>();
  for (const row of state.rows) {
    for (const tile of row.tiles) {
      for (const pkg of flattenPackages(tile.overview.packages ?? [])) {
        for (const file of pkg.files ?? []) files.set(`${pkg.directory}:${pkg.name}:${file.path}`, { packageDirectory: pkg.directory, packageName: pkg.name, file });
      }
      for (const file of tile.overview.files ?? []) {
        files.set(`${tile.target.packagePath ?? ''}:${tile.target.packageName ?? ''}:${file.path}`, { packageDirectory: tile.target.packagePath ?? '', packageName: tile.target.packageName ?? '', file });
      }
      if (tile.target.filePath && (tile.target.kind === 'file' || tile.target.kind === 'declaration')) {
        const file = {
          name: tile.target.filePath.split('/').pop() ?? tile.target.filePath,
          path: tile.target.filePath,
          declarations: tile.overview.declarations,
        };
        files.set(`${tile.target.packagePath ?? ''}:${tile.target.packageName ?? ''}:${file.path}`, {
          packageDirectory: tile.target.packagePath ?? '',
          packageName: tile.target.packageName ?? '',
          file,
        });
      }
    }
  }
  return Array.from(files.values());
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

function RevisionControls({ context, current, compare, compareSelections, workspace, canGenerate, canGenerateProposals, onCurrentChange, onCompareSelectionsChange, onGenerate, onGenerateProposals }: { context: RevisionContext | null; current: string; compare: string; compareSelections: string[]; workspace: string; canGenerate: boolean; canGenerateProposals: boolean; onCurrentChange: (value: string) => void; onCompareSelectionsChange: (values: string[]) => void; onGenerate: () => void; onGenerateProposals: () => void }) {
  const [pickerOpen, setPickerOpen] = useState(false);
  if (!context) return null;
  const options: RevisionOption[] = [{ kind: 'working-tree', ref: 'Working tree', hash: '', shortHash: '', date: '', subject: 'Current files on disk' }, ...context.options];
  const label = (option: RevisionOption) => option.kind === 'working-tree' ? option.ref : `${option.ref} · ${option.shortHash} · ${formatRevisionDate(option.date)}${option.subject ? ` · ${option.subject}` : ''}`;
  const viewing = workspace === compare && compare ? 'Compare to' : workspace === current ? 'Diff baseline' : 'Working AST';
  const compareOptions = context.options.filter((option) => option.hash && option.hash !== current);
  const toggleRevision = (revision: string) => {
    const next = compareSelections.includes(revision) ? compareSelections.filter((value) => value !== revision) : [...compareSelections, revision];
    onCompareSelectionsChange(next);
  };
  const selectedOptions = compareOptions.filter((option) => compareSelections.includes(option.hash));
  return <section className="revision-context" aria-label="Revision comparison"><div className="git-status">Git: {context.branch || 'detached'} · {context.currentCommit ? `at ${context.currentCommit.slice(0, 7)}` : 'no commits yet'}</div><div className="comparison-direction">Viewing: {viewing}. Apply/remove creates a projected working AST.</div><div><label htmlFor="current-revision">Diff baseline</label><select id="current-revision" value={current} onChange={(event) => onCurrentChange(event.target.value)}>{options.map((option) => <option key={`current:${option.kind}:${option.hash || option.ref}`} value={option.kind === 'working-tree' ? 'working-tree' : option.hash}>{label(option)}</option>)}</select></div><div className="compare-picker"><label htmlFor="compare-revisions">Compare to</label><div className="compare-select" id="compare-revisions"><div className="compare-select-control" role="combobox" aria-expanded={pickerOpen} aria-controls="compare-revision-options" tabIndex={0} onClick={() => setPickerOpen((open) => !open)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') setPickerOpen((open) => !open); }}>{selectedOptions.length === 0 && <span className="compare-select-placeholder">Choose revisions</span>}{selectedOptions.map((option) => <span className="compare-chip" key={option.hash}><span>{option.ref} <small>{option.shortHash}</small></span><button type="button" aria-label={`Remove ${option.ref}`} onClick={(event) => { event.stopPropagation(); toggleRevision(option.hash); }}>×</button></span>)}<span className="compare-select-chevron">▾</span></div>{pickerOpen && <div className="compare-picker-menu" id="compare-revision-options" role="listbox" aria-multiselectable="true">{compareOptions.map((option) => <button type="button" role="option" aria-selected={compareSelections.includes(option.hash)} className={compareSelections.includes(option.hash) ? 'compare-option selected' : 'compare-option'} key={`compare:${option.kind}:${option.hash}`} onClick={(event) => { event.stopPropagation(); toggleRevision(option.hash); }}><span>{option.ref}</span><small>{option.shortHash} · {formatRevisionDate(option.date)}</small>{compareSelections.includes(option.hash) && <strong>✓</strong>}</button>)}</div>}</div></div><button type="button" className="compare-placeholder" disabled={!canGenerate} onClick={onGenerate}>Generate edits</button><button type="button" className="compare-placeholder proposal-generate-button" disabled={!canGenerateProposals} onClick={onGenerateProposals}>{canGenerateProposals ? 'Generate proposal edits' : 'No new proposal sources'}</button></section>;
}

function ProposalControls({ workspace, disabled, proposalMode, onToggle, onReview }: { workspace: ProposalWorkspace; disabled: boolean; proposalMode: boolean; onToggle: (branchID: string) => void; onReview: () => void }) {
  const selected = new Set(workspace.selectedProposalIds ?? []);
  return <section className="proposal-controls" aria-label="Proposal branches"><div><label>{proposalMode ? 'Proposal targets' : 'Existing proposals'}</label><div className="proposal-list">{workspace.branches.filter((branch) => branch.kind === 'proposal').map((branch) => <label className="proposal-option" key={branch.id}><input type="checkbox" checked={selected.has(branch.id)} disabled={disabled || proposalMode} onChange={() => onToggle(branch.id)} />{branch.name}</label>)}{workspace.branches.filter((branch) => branch.kind === 'proposal').length === 0 && <span className="proposal-count">No proposals yet</span>}</div></div>{!proposalMode && selected.size > 0 && <button type="button" className="compare-placeholder" disabled={disabled} onClick={onReview}>Review selected proposals</button>}</section>;
}

function ProposalTabs({ proposals, activeBranchId, onSelect }: { proposals: ProposalWorkspace['branches']; activeBranchId: string; onSelect: (branchID: string) => void }) {
  return <nav className="proposal-tabs" aria-label="Proposal edit targets" role="tablist">{proposals.map((proposal) => <button type="button" role="tab" aria-selected={activeBranchId === proposal.id} className={activeBranchId === proposal.id ? 'proposal-tab active' : 'proposal-tab'} key={proposal.id} onClick={() => onSelect(proposal.id)}>{proposal.name} <small>edits</small></button>)}</nav>;
}

function formatRevisionDate(value: string): string {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export function buildComparisonTree(edits: EditSummary[], scope?: Declaration, excludedScopes: Declaration[] = []): ComparisonTreeNode[] {
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
    const ancestors = (edit.ancestors ?? (edit.ancestorIds ?? (edit.parentId ? [edit.parentId] : [])).map((nodeId) => ({ nodeId, nodeKind: nodeId === edit.parentId ? (edit.parentKind ?? 'AST node') : 'AST node' }))).filter((ancestor) => (!scope || !ancestor.startLine || !ancestor.endLine || (ancestor.startLine >= scope.line && ancestor.endLine <= scope.endLine)) && !excludedScopes.some((excluded) => editBelongsToDeclaration({ startLine: ancestor.startLine, endLine: ancestor.endLine, nodeKind: ancestor.nodeKind } as EditSummary, excluded) || declarationNodeKind(ancestor.nodeKind, excluded)));
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
  return roots.flatMap(compactComparisonTreeNode);
}

function compactComparisonTreeNode(node: ComparisonTreeNode): ComparisonTreeNode[] {
  node.children = node.children.flatMap(compactComparisonTreeNode);
  if (node.edit || node.children.length === 0) return [node];

  // Keep the nearest context that contains the actions, but remove the long
  // single-child and context-only ancestor chains above it.
  if (node.children.length === 1 && !node.children[0].edit) {
    return [node.children[0]];
  }
  if (node.children.length > 1 && node.children.every((child) => !child.edit)) {
    return node.children;
  }
  return [node];
}

type RowProps = {
  row: InquiryRow;
  editInquiries: EditInquiry[];
  comparisonPreview?: FileEditState;
  formatRequests: Record<string, number>;
  onFormat: (tileID: string) => void;
  editsFor: (tile: Tile) => EditSummary[];
  editsForPath: (packageDirectory: string, packageName: string, filePath: string) => EditSummary[];
  workingCodeForPath: (packageDirectory: string, packageName: string, filePath: string) => FileEditState | undefined;
  proposalCodeForPath: (packageDirectory: string, packageName: string, filePath: string) => FileEditState | undefined;
  proposalPreview?: FileEditState;
  comparisonActive: boolean;
  completeFileByTile: Record<string, boolean>;
  onCompleteFileChange: (tileID: string, value: boolean) => void;
  onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary, fullLine?: boolean) => void;
  onCopyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void;
  canCopyProposal: boolean;
  onOpenComparisonFile: (file: ComparisonFile) => void;
  onLocateEdit: (file: ComparisonFile, edit: EditSummary) => void;
  onOpenEditColumn: (row: InquiryRow, tile: Tile, file: ComparisonFile) => void;
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

function InquiryRowView({ row, editInquiries, comparisonPreview, proposalPreview, formatRequests, onFormat, focus, editsFor, editsForPath, workingCodeForPath, proposalCodeForPath, comparisonActive, completeFileByTile, onCompleteFileChange, onApplyEdit, onCopyEdit, canCopyProposal, onOpenComparisonFile, onLocateEdit, onOpenEditColumn, textRefs, onPackage, onPackageColumn, onPackageLeft, onImport, onImportColumn, onImportLeft, onFile, onFileColumn, onFileLeft, onInspectFile, onInspectReference, onOpenReferenceLeft, onOpenReferenceColumn, onDeclaration, onDeclarationColumn, onDeclarationLeft, onInspectDeclaration, onLensChange, packageLens, fileLens, onTogglePane, onToggleTile, onCloseColumn, onCloseTile, onBack }: RowProps) {
  return <section className="inquiry-row"><div className="row-heading"><span>{row.title ?? 'Inquiry'}</span><span className="row-count">{row.tiles.length} tile{row.tiles.length === 1 ? '' : 's'}</span></div><div className="tile-strip">
    {row.tiles.map((tile) => <article className="tile" key={tile.id}>
       <div className="tile-meta"><span>Column {tile.column + 1}</span><span className="tile-meta-actions">{tile.openedBy && <span className="relationship">{tile.openedBy.relationship}</span>}<button type="button" className="close-tile" onClick={() => onCloseTile(row, tile)} disabled={row.tiles.length === 1} aria-label={`Close column ${tile.column + 1}`}>Close</button><button type="button" className="close-tile" onClick={() => onBack(row, tile)} disabled={!tile.canGoBack}>Back</button><button type="button" className="close-tile" onClick={() => onToggleTile(row.id, tile)} aria-expanded={!tile.collapsed}>{tile.collapsed ? 'Open' : 'Collapse'}</button></span></div>
      {tile.previouslyOpened && <div className="cycle-note">Previously opened earlier in this inquiry.</div>}
      {!tile.collapsed && <div className="tile-views">
        <section className={`tile-view overview-view ${tile.panes.overviewCollapsed ? 'collapsed' : ''}`}>
          <button type="button" className="pane-heading" onClick={() => onTogglePane(row.id, tile, 'overview')}><span>Overview</span><span>{tile.panes.overviewCollapsed ? '+' : '−'}</span></button>
           {!tile.panes.overviewCollapsed && <Overview tile={tile} row={row} editInquiries={tile.target.kind === 'program' && row.title === 'Program inquiry' ? editInquiries : []} edits={editsFor(tile)} comparisonActive={comparisonActive} completeFile={completeFileByTile[tile.id] === true} onCompleteFileChange={(value) => onCompleteFileChange(tile.id, value)} onApplyEdit={onApplyEdit} onCopyEdit={onCopyEdit} canCopyProposal={canCopyProposal} onOpenComparisonFile={onOpenComparisonFile} onLocateEdit={onLocateEdit} onOpenEditColumn={onOpenEditColumn} editsForPath={editsForPath} packageLens={packageLens(tile)} fileLens={fileLens(tile)} onLensChange={onLensChange} onPackage={onPackage} onPackageColumn={onPackageColumn} onPackageLeft={onPackageLeft} onImport={onImport} onImportColumn={onImportColumn} onImportLeft={onImportLeft} onFile={onFile} onFileColumn={onFileColumn} onFileLeft={onFileLeft} onInspectDeclaration={onInspectDeclaration} onDeclaration={onDeclaration} onDeclarationColumn={onDeclarationColumn} onDeclarationLeft={onDeclarationLeft} onInspectReference={onInspectReference} onOpenReferenceLeft={onOpenReferenceLeft} onOpenReferenceColumn={onOpenReferenceColumn} />}
        </section>
        <section className={`tile-view text-view ${tile.panes.textCollapsed ? 'collapsed' : ''}`}>
           <div className="pane-heading"><button type="button" className="pane-heading-toggle" onClick={() => onTogglePane(row.id, tile, 'text')}><span>Text representation</span><span>{tile.panes.textCollapsed ? '+' : '−'}</span></button>{tile.target.kind !== 'package' && <button type="button" className="format-button" onClick={() => onFormat(tile.id)} disabled={tile.panes.textCollapsed}>Format</button>}</div>
           {!tile.panes.textCollapsed && (tile.target.kind === 'package' ? <PackageSourcePane tile={tile} focus={focus} textRefs={textRefs} row={row} onInspectFile={onInspectFile} workingCodeForPath={workingCodeForPath} proposalCodeForPath={proposalCodeForPath} /> : <TextRepresentation tile={tile} focus={focus} textRefs={textRefs} editState={tile.target.kind === 'program' ? comparisonPreview : workingCodeForPath(tile.target.packagePath ?? '', tile.target.packageName ?? '', tile.target.filePath ?? '')} alternateEditState={tile.target.kind === 'program' ? proposalPreview : proposalCodeForPath(tile.target.packagePath ?? '', tile.target.packageName ?? '', tile.target.filePath ?? '')} alternateLabel="Immutable proposal" formatRequest={formatRequests[tile.id] ?? 0} />)}
        </section>
      </div>}
    </article>)}
  </div></section>;
}

function TextRepresentation({ tile, focus, textRefs, editState, alternateEditState, alternateLabel, formatRequest }: { tile: Tile; focus: FocusTarget | null; textRefs: React.MutableRefObject<Record<string, HTMLPreElement | null>>; editState?: FileEditState; alternateEditState?: FileEditState; alternateLabel?: string; formatRequest: number }) {
  const [showAlternate, setShowAlternate] = useState(false);
  useEffect(() => setShowAlternate(false), [alternateEditState?.branchId, editState?.workingCode]);
  const visibleEditState = showAlternate && alternateEditState ? alternateEditState : editState;
  const projectedEditState = visibleEditState && (hasProjectedEdits(visibleEditState) || visibleEditState.workingAuthoritative || !tile.text.content) ? visibleEditState : undefined;
  const rawContent = showAlternate && alternateEditState
    ? alternateEditState.targetCode ?? alternateEditState.workingCode
    : tile.target.kind === 'declaration'
      ? declarationContent(tile, projectedEditState)
      : projectedEditState
        ? projectedEditState.workingCode
        : tile.text.content;
  const [content, setContent] = useState(rawContent);
  const [formatError, setFormatError] = useState('');
  useEffect(() => {
    let active = true;
    if (!rawContent || (tile.text.language !== 'typescript' && tile.text.language !== 'tsx')) {
      setContent(rawContent);
      setFormatError('');
      return () => { active = false; };
    }
    setFormatError('');
    prettier.format(rawContent, {
      parser: tile.text.language === 'tsx' ? 'babel-ts' : 'typescript',
      plugins: [prettierBabel, prettierEstree, prettierTypescript],
      singleQuote: true,
      semi: true,
      tabWidth: 2,
      printWidth: 100,
      trailingComma: 'all',
      bracketSpacing: true,
      arrowParens: 'always',
    }).then((formatted) => {
      if (active) setContent(addReadableTypeScriptSpacing(formatted));
    }).catch((reason) => {
      if (active) {
        setContent(rawContent);
        setFormatError(reason instanceof Error ? reason.message : 'Prettier could not format this source.');
      }
    });
    return () => { active = false; };
  }, [rawContent, tile.text.language, formatRequest]);
  const diagnostics = [...(visibleEditState?.diagnostics ?? []), ...(visibleEditState?.renderDiagnostics ?? [])];
  if (!content) {
    return <><SourceVersionTabs showAlternate={showAlternate} alternateLabel={alternateLabel} onSelectAlternate={setShowAlternate} /><AstStatus editState={visibleEditState} /><pre ref={(element) => { textRefs.current[tile.id] = element; }}><code>No text representation for this target.</code></pre></>;
  }
  const lines = content.split('\n');
  const sourceStartLine = tile.text.sourceStartLine ?? 1;
  return <><SourceVersionTabs showAlternate={showAlternate} alternateLabel={alternateLabel} onSelectAlternate={setShowAlternate} /><AstStatus editState={visibleEditState} />{formatError && <div className="render-diagnostics">Prettier: {formatError}</div>}<pre ref={(element) => { textRefs.current[tile.id] = element; }}><code>{lines.map((line, index) => {
    const sourceLine = sourceStartLine + index;
    const matches = occurrencesForLine(tile.text.occurrences ?? [], focus, sourceLine);
    const focused = lineFocusMatches(tile, focus, sourceLine);
    const changed = !showAlternate && (visibleEditState?.edits.some((edit) => edit.startLine !== undefined && edit.endLine !== undefined && edit.startLine <= sourceLine && edit.endLine >= sourceLine) ?? false);
    return <span className={`source-line${matches.length > 0 || focused ? ' focused' : ''}${changed ? ' edit-location' : ''}`} data-focus-match={matches.length > 0 || focused ? 'true' : undefined} key={`${tile.id}:${index}`}>{highlightSource(line, matches, sourceLine)}{index < lines.length - 1 ? '\n' : ''}</span>;
  })}</code></pre>{diagnostics.length > 0 && <div className="render-diagnostics">{diagnostics.map((diagnostic, index) => <div key={`${tile.id}:diagnostic:${index}`}>{diagnostic}</div>)}</div>}</>;
}

function SourceVersionTabs({ showAlternate, alternateLabel, onSelectAlternate }: { showAlternate: boolean; alternateLabel?: string; onSelectAlternate: (value: boolean) => void }) {
  if (!alternateLabel) return null;
  return <div className="source-version-tabs" role="tablist" aria-label="Source versions"><button type="button" role="tab" aria-selected={!showAlternate} className={!showAlternate ? 'source-version-tab active' : 'source-version-tab'} onClick={() => onSelectAlternate(false)}>Human build</button><button type="button" role="tab" aria-selected={showAlternate} className={showAlternate ? 'source-version-tab active' : 'source-version-tab'} onClick={() => onSelectAlternate(true)}>{alternateLabel}</button></div>;
}

function addReadableTypeScriptSpacing(source: string): string {
  const lines = source.split('\n');
  const result: string[] = [];
  let importBlock = true;
  for (const line of lines) {
    const trimmed = line.trim();
    const topLevel = trimmed !== '' && !line.startsWith(' ') && !line.startsWith('\t');
    const startsDeclaration = /^(const|let|var|function|class|interface|type|export)\b/.test(trimmed);
    const startsTopLevelCall = /^[A-Za-z_$][\w$]*(?:\.|\s*\()/.test(trimmed) && !/^(if|for|while|switch|catch)\b/.test(trimmed);
    const previous = result[result.length - 1]?.trim() ?? '';
    const previousIsDeclaration = /^(const|let|var)\b/.test(previous);
    const needsSectionBreak = topLevel && result.length > 0 && result[result.length - 1] !== '' && (
      (importBlock && startsDeclaration) ||
      (startsTopLevelCall && previousIsDeclaration) ||
      (startsTopLevelCall && previous.endsWith('});'))
    );
    if (needsSectionBreak) {
      result.push('');
    }
    if (topLevel && !trimmed.startsWith('import ')) {
      importBlock = false;
    }
    result.push(line);
  }
  return result.join('\n');
}

function declarationContent(tile: Tile, editState?: FileEditState): string | undefined {
  const declaration = tile.overview.declarations?.[0];
  const source = editState && (hasProjectedEdits(editState) || !tile.text.content) ? editState.workingCode : tile.text.content;
  if (!declaration || !source) return source;
  const lines = source.split('\n');
  const start = declarationStartLine(lines, declaration);
  let depth = 0;
  let opened = false;
  for (let index = start; index < lines.length; index += 1) {
    for (const character of lines[index]) {
      if (character === '{') {
        depth += 1;
        opened = true;
      } else if (character === '}') {
        depth -= 1;
      }
    }
    if (opened && depth <= 0) return lines.slice(start, index + 1).join('\n');
  }
  return lines.slice(start, declaration.endLine).join('\n');
}

function hasProjectedEdits(editState: FileEditState): boolean {
  return editState.edits.some((edit) => edit.status === 'applied' || edit.status === 'prepared' || edit.status === 'removed');
}

function lineFocusMatches(tile: Tile, focus: FocusTarget | null, line: number): boolean {
  if (focus?.kind !== 'line') return false;
  const filePath = tile.target.filePath ?? tile.text.filename ?? '';
  return focus.path === filePath && focus.startLine <= line && focus.endLine >= line;
}

function declarationStartLine(lines: string[], declaration: Declaration): number {
  const name = declaration.name;
  const index = lines.findIndex((line) => {
    if (!line.includes(name)) return false;
    if (declaration.kind === 'function' || declaration.kind === 'method') return /\bfunc\b/.test(line);
    return new RegExp(`\\b${declaration.kind}\\b`).test(line);
  });
  return index >= 0 ? index : Math.max(0, declaration.line - 1);
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

function Overview({ tile, row, editInquiries, edits, comparisonActive, completeFile, onCompleteFileChange, onApplyEdit, onCopyEdit, canCopyProposal, onOpenComparisonFile, onLocateEdit, onOpenEditColumn, editsForPath, packageLens, fileLens, onLensChange, onPackage, onPackageColumn, onPackageLeft, onImport, onImportColumn, onImportLeft, onFile, onFileColumn, onFileLeft, onInspectDeclaration, onDeclaration, onDeclarationColumn, onDeclarationLeft, onInspectReference, onOpenReferenceLeft, onOpenReferenceColumn }: Omit<RowProps, 'onTogglePane' | 'onToggleTile' | 'onCloseColumn' | 'onCloseTile' | 'onBack' | 'focus' | 'textRefs' | 'onInspectFile' | 'onInspectReference' | 'onOpenReferenceLeft' | 'onOpenReferenceColumn' | 'editsFor' | 'editsForPath' | 'workingCodeForPath' | 'proposalCodeForPath' | 'proposalPreview' | 'packageLens' | 'fileLens' | 'comparisonActive' | 'comparisonPreview' | 'completeFileByTile' | 'onCompleteFileChange' | 'comparisonRows' | 'onApplyEdit' | 'onCopyEdit' | 'canCopyProposal' | 'onOpenComparisonFile' | 'onLocateEdit' | 'onOpenEditColumn' | 'editInquiries'> & { tile: Tile; editInquiries: EditInquiry[]; edits: EditSummary[]; comparisonActive: boolean; completeFile: boolean; onCompleteFileChange: (value: boolean) => void; onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary, fullLine?: boolean) => void; onCopyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void; canCopyProposal: boolean; onOpenComparisonFile: (file: ComparisonFile) => void; onLocateEdit: (file: ComparisonFile, edit: EditSummary) => void; onOpenEditColumn: (row: InquiryRow, tile: Tile, file: ComparisonFile) => void; editsForPath: (packageDirectory: string, packageName: string, filePath: string) => EditSummary[]; packageLens: PackageLens; fileLens: FileLens; onInspectReference: (row: InquiryRow, tile: Tile, reference: Reference) => void; onOpenReferenceLeft: (row: InquiryRow, tile: Tile, reference: Reference) => void; onOpenReferenceColumn: (row: InquiryRow, tile: Tile, reference: Reference) => void }) {
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
  const packageInternalRows = (file: File): Array<{ declaration: Declaration; child: boolean }> => declarationsFor(file).flatMap((declaration) => {
    if (declaration.exported) return [];
    return [{ declaration, child: false }, ...(declaration.children ?? []).filter((child) => !child.exported).map((child) => ({ declaration: child, child: true }))];
  });
  const editDeclaration = overview.selectedDeclaration ?? (tile.target.kind === 'declaration' ? overview.declarations?.[0] : undefined);
  const openedDeclarations = row.tiles.filter((candidate) => candidate.id !== tile.id && candidate.target.kind === 'declaration' && candidate.target.packagePath === tile.target.packagePath && candidate.target.packageName === tile.target.packageName && candidate.target.filePath === tile.target.filePath).flatMap((candidate) => candidate.overview.declarations ?? []);
  const visibleComparisonEdits = tile.target.kind === 'file' ? edits.filter((edit) => !openedDeclarations.some((declaration) => editBelongsToDeclaration(edit, declaration))) : edits;
  return <div className="overview-content"><h2>{overview.title}</h2>{overview.subtitle && <p className="subtitle">{overview.subtitle}</p>}
      {tile.target.kind === 'package' && <LensControls value={packageLens} options={[['files', 'Files'], ['api', 'Exported API'], ['internal', 'Internal API']]} onChange={(value) => onLensChange(tile, value as PackageLens)} />}
     {tile.target.kind === 'file' && <LensControls value={fileLens} options={[['all', 'All'], ['exported', 'Exported'], ['internal', 'Internal']]} onChange={(value) => onLensChange(tile, value as FileLens)} />}
        {overview.packages?.map((pkg) => <TargetButton key={`${pkg.directory}:${pkg.name}`} hasEdits={packageHasEdits(pkg, editsForPath)} label={`${pkg.kind === 'folder' ? 'folder' : pkg.kind === 'project' ? 'project' : 'package'} ${pkg.name}`} detail={`${pkg.directory || 'project root'} · ${pkg.fileCount} files`} canOpenLeft={tile.column > 0} onOpen={() => onPackage(row, tile, pkg)} onOpenLeft={() => onPackageLeft(row, tile, pkg)} onOpenColumn={() => onPackageColumn(row, tile, pkg)} />)}
     {tile.target.kind === 'program' && editInquiries.length > 0 && <StructuralEditsSection row={row} tile={tile} inquiries={editInquiries} onApplyEdit={onApplyEdit} onCopyEdit={onCopyEdit} canCopyProposal={canCopyProposal} onOpenComparisonFile={onOpenComparisonFile} onLocateEdit={onLocateEdit} onOpenEditColumn={onOpenEditColumn} onOpenDeclaration={(file, declaration) => onDeclarationColumn(row, tile, file, declaration)} />}
      {tile.target.kind === 'package' && packageLens === 'files' && overview.files?.map((file) => <TargetButton key={file.path} hasEdits={editsForPath(tile.target.packagePath ?? '', tile.target.packageName ?? '', file.path).length > 0} label={file.name} detail={file.path} canOpenLeft={tile.column > 0} onOpen={() => onFile(row, tile, file)} onOpenLeft={() => onFileLeft(row, tile, file)} onOpenColumn={() => onFileColumn(row, tile, file)} />)}
      {tile.target.kind === 'package' && (packageLens === 'api' || packageLens === 'internal') && overview.files?.flatMap((file) => (packageLens === 'api' ? packageApiRows(file) : packageInternalRows(file)).map(({ declaration, child }) => <TargetButton key={`${file.path}:${declaration.symbolId ?? declaration.name}:${declaration.line}`} label={`${child ? '↳ ' : ''}${declaration.kind} ${declaration.name}`} detail={`${file.name} · ${declarationSignature(declaration)} · line ${declaration.line}`} canOpenLeft={tile.column > 0} onOpen={() => onInspectDeclaration(row, tile, file.path, declaration)} onOpenLeft={() => onDeclarationLeft(row, tile, file.path, declaration)} onOpenColumn={() => onDeclarationColumn(row, tile, file.path, declaration)} />))}
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
       }).map(({ declaration, child }) => <TargetButton key={`${declaration.symbolId ?? declaration.name}:${declaration.line}`} hasEdits={editsForDeclaration(edits, declaration).length > 0} label={`${child ? '↳ ' : ''}${declaration.kind} ${declaration.name}`} detail={`${declarationSignature(declaration)} · line ${declaration.line}`} canOpenLeft={tile.column > 0} onOpen={() => onDeclaration(row, tile, declaration)} onOpenLeft={() => onDeclarationLeft(row, tile, tile.target.filePath ?? '', declaration)} onOpenColumn={() => onDeclarationColumn(row, tile, tile.target.filePath ?? '', declaration)} />)}
       {(tile.target.kind === 'declaration' || overview.selectedDeclaration) && (overview.references?.length ?? 0) > 0 && <section className="overview-section references-section"><h3>{overview.selectedDeclaration ? `${overview.selectedDeclaration.kind} ${overview.selectedDeclaration.name}` : 'References'}</h3><button type="button" className="section-toggle" onClick={() => setShowReferences((visible) => !visible)}>{showReferences ? 'Hide references' : `Find references (${overview.references?.length})`}</button>{showReferences && overview.references?.map((reference) => <ReferenceButton key={`${reference.filePath}:${reference.referenceLine ?? reference.line}`} reference={reference} canOpenLeft={tile.column > 0} onOpen={() => onInspectReference(row, tile, reference)} onOpenLeft={() => onOpenReferenceLeft(row, tile, reference)} onOpenColumn={() => onOpenReferenceColumn(row, tile, reference)} />)}</section>}
          {comparisonActive && (tile.target.kind === 'file' || tile.target.kind === 'declaration') ? <ComparisonTreeSection tile={tile} edits={visibleComparisonEdits} excludedDeclarations={openedDeclarations} completeFile={completeFile} onCompleteFileChange={onCompleteFileChange} onApplyEdit={onApplyEdit} onCopyEdit={onCopyEdit} canCopyProposal={canCopyProposal} onLocateEdit={onLocateEdit} onOpenDeclaration={(declaration) => onDeclarationColumn(row, tile, tile.target.filePath ?? '', declaration)} onOpenDeclarationLeft={(declaration) => onDeclarationLeft(row, tile, tile.target.filePath ?? '', declaration)} /> : editsForDeclaration(edits, editDeclaration).length > 0 && <EditSection edits={editsForDeclaration(edits, editDeclaration)} />}
    </div>;
}

function StructuralEditsSection({ row, tile, inquiries, onApplyEdit, onCopyEdit, canCopyProposal, onOpenComparisonFile, onLocateEdit, onOpenEditColumn, onOpenDeclaration }: { row: InquiryRow; tile: Tile; inquiries: EditInquiry[]; onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary, fullLine?: boolean) => void; onCopyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void; canCopyProposal: boolean; onOpenComparisonFile: (file: ComparisonFile) => void; onLocateEdit: (file: ComparisonFile, edit: EditSummary) => void; onOpenEditColumn: (row: InquiryRow, tile: Tile, file: ComparisonFile) => void; onOpenDeclaration: (file: string, declaration: Declaration) => void }) {
  const editCount = inquiries.reduce((count, inquiry) => count + inquiry.columns.reduce((columnCount, column) => columnCount + column[0].edits.length, 0), 0);
  const sequence = inquiries.flatMap((inquiry) => inquiry.columns.flatMap((column) => topLevelEditGroups(buildComparisonTree(column[0].edits)).map((edits) => ({ edits, file: column[0] }))));
  return <section className="overview-section edit-section comparison-results structural-edits"><div className="comparison-tree-heading"><div><h3>Structural edits ({editCount})</h3><p>Revision changes in the Program inquiry. Highlight or open any edit in the baseline source.</p></div><div><span className="comparison-scope">Tree</span><EditSequenceControls groups={sequence.map(({ edits }) => edits)} onApply={(edits) => { const item = sequence.find((candidate) => candidate.edits.includes(edits[0])); if (item) edits.forEach((edit) => onApplyEdit(item.file.packageDirectory, item.file.packageName, item.file.path, edit, true)); }} /></div></div><div className="comparison-tree">{inquiries.flatMap((inquiry) => inquiry.columns.map((column, index) => <StructuralEditGroup key={`${inquiry.id}:${index}`} row={row} tile={tile} inquiry={inquiry} file={column[0]} index={index} onApplyEdit={onApplyEdit} onCopyEdit={onCopyEdit} canCopyProposal={canCopyProposal} onOpenComparisonFile={onOpenComparisonFile} onLocateEdit={onLocateEdit} onOpenEditColumn={onOpenEditColumn} onOpenDeclaration={onOpenDeclaration} />))}</div></section>;
}

function StructuralEditGroup({ row, tile, inquiry, file, index, onApplyEdit, onCopyEdit, canCopyProposal, onOpenComparisonFile, onLocateEdit, onOpenEditColumn, onOpenDeclaration }: { row: InquiryRow; tile: Tile; inquiry: EditInquiry; file: ComparisonFile; index: number; onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary, fullLine?: boolean) => void; onCopyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void; canCopyProposal: boolean; onOpenComparisonFile: (file: ComparisonFile) => void; onLocateEdit: (file: ComparisonFile, edit: EditSummary) => void; onOpenEditColumn: (row: InquiryRow, tile: Tile, file: ComparisonFile) => void; onOpenDeclaration: (file: string, declaration: Declaration) => void }) {
  const [expanded, setExpanded] = useState(Boolean(inquiry.declaration));
  const tree = buildComparisonTree(file.edits);
  const initialExpanded = new Set(pathToFirstEdit(tree));
  const label = `${inquiry.title}${inquiry.columns.length > 1 ? ` · branch ${index + 1}` : ''}`;
  return <div className="structural-edit-group"><div className="structural-edit-label"><button type="button" className="structural-edit-toggle" onClick={() => setExpanded((value) => !value)} aria-expanded={expanded}>{expanded ? '−' : '+'}</button>{inquiry.declaration ? <button type="button" className="structural-edit-open" onClick={() => onOpenDeclaration(file.path, inquiry.declaration)}>{label}</button> : <span>{label}</span>}{inquiry.title !== file.path && <span>{file.path}</span>}</div><div className="edit-navigation-actions"><button type="button" className="open-inquiry-button file-inquiry-button" onClick={() => onOpenComparisonFile(file)}>Open source</button><button type="button" className="open-inquiry-button file-inquiry-button" onClick={() => onOpenEditColumn(row, tile, file)}>Open in column</button></div>{expanded && tree.map((node) => <ComparisonTreeNodeView key={`${inquiry.id}:${index}:${node.id}`} node={node} depth={0} initialExpanded={initialExpanded} packageDirectory={file.packageDirectory} packageName={file.packageName} file={file.path} onApplyEdit={onApplyEdit} onCopyEdit={onCopyEdit} canCopyProposal={canCopyProposal} onLocateEdit={(edit) => onLocateEdit(file, edit)} />)}</div>;
}

function ComparisonTreeSection({ tile, edits, excludedDeclarations, completeFile, onCompleteFileChange, onApplyEdit, onCopyEdit, canCopyProposal, onLocateEdit, onOpenDeclaration, onOpenDeclarationLeft }: { tile: Tile; edits: EditSummary[]; excludedDeclarations: Declaration[]; completeFile: boolean; onCompleteFileChange: (value: boolean) => void; onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary, fullLine?: boolean) => void; onCopyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void; canCopyProposal: boolean; onLocateEdit: (file: ComparisonFile, edit: EditSummary) => void; onOpenDeclaration: (declaration: Declaration) => void; onOpenDeclarationLeft: (declaration: Declaration) => void }) {
  const declaration = tile.target.kind === 'declaration' ? tile.overview.declarations?.[0] : undefined;
  const scopedEdits = declaration ? edits.filter((edit) => editBelongsToDeclaration(edit, declaration)) : edits;
  const tree = buildComparisonTree(scopedEdits, declaration, excludedDeclarations);
  const initialExpanded = new Set(pathToFirstEdit(tree));
  const declarations = tile.target.kind === 'file' ? (tile.overview.declarations ?? []).filter((declaration) => editsForDeclaration(edits, declaration).length > 0) : [];
  const file: ComparisonFile = { packageDirectory: tile.target.packagePath ?? '', packageName: tile.target.packageName ?? '', path: tile.target.filePath ?? '', edits: scopedEdits, declaration };
  return <section className="overview-section edit-section comparison-results"><div className="comparison-tree-heading"><div><h3>Compared edits ({scopedEdits.length})</h3><p>{declaration ? `Edits inside ${declaration.kind} ${declaration.name}.` : 'Complete canonical edit tree. Structural replacements apply together.'}</p></div><div className="comparison-heading-actions"><span className="comparison-scope">{declaration ? 'Declaration' : 'Complete file'}</span><EditSequenceControls groups={topLevelEditGroups(tree)} onApply={(edits) => edits.forEach((edit) => onApplyEdit(tile.target.packagePath ?? '', tile.target.packageName ?? '', tile.target.filePath ?? '', edit, true))} />{declarations.map((declaration) => <span className="declaration-inquiry-actions" key={`${declaration.symbolId ?? declaration.name}:${declaration.line}`}><span>{declaration.kind} {declaration.name}</span><button type="button" className="open-inquiry-button" onClick={() => onOpenDeclarationLeft(declaration)}>Open left</button><button type="button" className="open-inquiry-button" onClick={() => onOpenDeclaration(declaration)}>Open right</button></span>)}</div></div>{tree.length === 0 ? <p className="empty-note">No edits in this scope.</p> : <div className="comparison-tree">{tree.map((node) => <ComparisonTreeNodeView key={node.id} node={node} depth={0} initialExpanded={initialExpanded} packageDirectory={tile.target.packagePath ?? ''} packageName={tile.target.packageName ?? ''} file={tile.target.filePath ?? ''} onApplyEdit={onApplyEdit} onCopyEdit={onCopyEdit} canCopyProposal={canCopyProposal} onLocateEdit={(edit) => onLocateEdit(file, edit)} />)}</div>}</section>;
}

function pathToFirstEdit(nodes: ComparisonTreeNode[]): string[] {
  for (const node of nodes) {
    if (node.edit) return [];
    const childPath = pathToFirstEdit(node.children);
    if (childPath.length > 0 || node.children.some((child) => child.edit)) return [node.id, ...childPath];
  }
  return [];
}

function firstUnappliedDescendantEdit(node: ComparisonTreeNode): EditSummary | undefined {
  if (node.edit && !node.edit.proposalApplied && node.edit.status !== 'applied' && node.edit.status !== 'prepared') return node.edit;
  for (const child of node.children) {
    const edit = firstUnappliedDescendantEdit(child);
    if (edit) return edit;
  }
  return undefined;
}

function topLevelEditGroups(nodes: ComparisonTreeNode[]): EditSummary[][] {
  return nodes.flatMap((root) => {
    if (root.edit) return [[firstUnappliedDescendantEdit(root)].filter((edit): edit is EditSummary => Boolean(edit))];
    const children = root.children.length > 0 ? root.children : [root];
    return children.map(firstUnappliedDescendantEdit).filter((edit): edit is EditSummary => Boolean(edit)).map((edit) => [edit]);
  }).filter((group) => group.length > 0);
}

function EditSequenceControls({ groups, onApply, disabled = false }: { groups: EditSummary[][]; onApply: (edits: EditSummary[]) => void; disabled?: boolean }) {
  const [playing, setPlaying] = useState(false);
  const stopRef = useRef(false);
  const groupsRef = useRef(groups);
  groupsRef.current = groups;
  const pending = groups.filter((group) => group.length > 0);
  useEffect(() => () => { stopRef.current = true; }, []);
  const signature = (items: EditSummary[][]) => items.map((group) => group.map((edit) => `${edit.index}:${edit.status}:${edit.proposalApplied ? 'applied' : 'pending'}`).join(',')).join('|');
  const play = async () => {
    if (playing) return;
    stopRef.current = false;
    setPlaying(true);
    while (!stopRef.current) {
      const currentGroups = groupsRef.current.filter((group) => group.length > 0);
      if (currentGroups.length === 0) break;
      const before = signature(currentGroups);
      if (stopRef.current) break;
      onApply(currentGroups[0]);
      await new Promise((resolve) => window.setTimeout(resolve, 500));
      let waited = 0;
      while (!stopRef.current && signature(groupsRef.current) === before && waited < 5000) {
        await new Promise((resolve) => window.setTimeout(resolve, 100));
        waited += 100;
      }
    }
    setPlaying(false);
  };
  const stop = () => {
    stopRef.current = true;
    setPlaying(false);
  };
  return <div className="edit-sequence-controls"><button type="button" className="open-inquiry-button" disabled={disabled || playing || pending.length === 0} onClick={() => onApply(pending[0])}>Apply next</button>{playing ? <button type="button" className="open-inquiry-button sequence-stop-button" onClick={stop}>Stop</button> : <button type="button" className="open-inquiry-button" disabled={disabled || pending.length === 0} onClick={() => void play()}>Play all slowly</button>}<span className="sequence-count">{pending.length} groups pending</span></div>;
}

function ComparisonTreeNodeView({ node, depth, initialExpanded, packageDirectory, packageName, file, onApplyEdit, onCopyEdit, canCopyProposal, onLocateEdit }: { node: ComparisonTreeNode; depth: number; initialExpanded?: Set<string>; packageDirectory: string; packageName: string; file: string; onApplyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary, fullLine?: boolean) => void; onCopyEdit: (packageDirectory: string, packageName: string, file: string, edit: EditSummary) => void; canCopyProposal: boolean; onLocateEdit: (edit: EditSummary) => void }) {
  const [expanded, setExpanded] = useState(initialExpanded?.has(node.id) ?? false);
  const edit = node.edit;
  const groupEdit = edit ? undefined : firstUnappliedDescendantEdit(node);
  const fullLineAction = edit && node.children.length > 0
    ? edit.status === 'applied' || edit.status === 'prepared' ? 'Remove full line' : 'Apply full line'
    : undefined;
  return <div className="comparison-tree-node"><div className={`comparison-tree-item ${edit ? 'comparison-tree-edit' : 'comparison-tree-context'}`} style={{ marginLeft: `${depth * 1.1}rem` }}><button type="button" className="tree-toggle" onClick={() => setExpanded((value) => !value)} disabled={node.children.length === 0} aria-label={node.children.length === 0 ? 'Leaf node' : expanded ? 'Collapse node' : 'Expand node'}>{node.children.length === 0 ? '·' : expanded ? '−' : '+'}</button><span className="tree-node-label">{edit && <span className="tree-edit-kind">{edit.kind}</span>}<strong>{node.nodeKind.replace('*ast.', '')}</strong>{node.field && <span className="tree-field">{node.field}</span>}{node.startLine !== undefined && <span className="tree-location">line {node.startLine}{node.endLine !== undefined && node.endLine !== node.startLine ? `-${node.endLine}` : ''}</span>}{node.value && <code>{node.value}</code>}</span>{edit && <button type="button" className="edit-action locate-action" onClick={() => onLocateEdit(edit)}>Locate</button>}{!edit && groupEdit && <button type="button" className="edit-action group-apply-action" onClick={() => onApplyEdit(packageDirectory, packageName, file, groupEdit, true)}>Apply group</button>}{canCopyProposal && edit && <button type="button" className="edit-action proposal-copy-action" onClick={() => onCopyEdit(packageDirectory, packageName, file, edit)}>Copy to human build</button>}{fullLineAction && !edit?.proposalApplied && <button type="button" className="edit-action full-line-action" onClick={() => onApplyEdit(packageDirectory, packageName, file, edit, true)}>{fullLineAction}</button>}{edit && (edit.proposalApplied ? <span className="edit-action applied-edit-status">Applied to Human build</span> : <button type="button" className="edit-action" onClick={() => onApplyEdit(packageDirectory, packageName, file, edit)} disabled={edit.status === 'prepared'}>{edit.status === 'applied' || edit.status === 'prepared' ? 'Remove' : edit.kind === 'DELETE' ? 'Apply replacement' : 'Apply'}</button>)}</div>{expanded && node.children.map((child) => <ComparisonTreeNodeView key={child.id} node={child} depth={depth + 1} initialExpanded={initialExpanded} packageDirectory={packageDirectory} packageName={packageName} file={file} onApplyEdit={onApplyEdit} onCopyEdit={onCopyEdit} canCopyProposal={canCopyProposal} onLocateEdit={onLocateEdit} />)}</div>;
}

function packageHasEdits(pkg: Package, editsForPath: (packageDirectory: string, packageName: string, filePath: string) => EditSummary[]): boolean {
  return packageEditCount(pkg, editsForPath) > 0;
}

function packageEditCount(pkg: Package, editsForPath: (packageDirectory: string, packageName: string, filePath: string) => EditSummary[]): number {
  return (pkg.files ?? []).reduce((count, file) => count + editsForPath(pkg.directory, pkg.name, file.path).length, 0)
    + (pkg.children ?? []).reduce((count, child) => count + packageEditCount(child, editsForPath), 0);
}

function editsForDeclaration(edits: EditSummary[], declaration?: Declaration): EditSummary[] {
  if (!declaration) return edits;
  return edits.filter((edit) => !edit.startLine || !edit.endLine || (edit.startLine <= declaration.endLine && edit.endLine >= declaration.line));
}

function editBelongsToDeclaration(edit: EditSummary, declaration: Declaration): boolean {
  if (edit.ancestors?.some((ancestor) => declarationNodeKind(ancestor.nodeKind, declaration))) return true;
  if (edit.startLine === undefined || edit.endLine === undefined || edit.startLine < declaration.line || edit.endLine > declaration.endLine) return false;
  if (edit.startLine === declaration.line && edit.endLine === declaration.line && !edit.nodeKind.replace('*ast.', '').toLowerCase().includes(declaration.kind.toLowerCase())) return false;
  return true;
}

function declarationNodeKind(nodeKind: string, declaration: Declaration): boolean {
  const normalized = nodeKind.replace('*ast.', '').toLowerCase();
  const expected = declaration.kind === 'function' || declaration.kind === 'method' ? 'funcdecl' : declaration.kind === 'type' ? 'typespec' : declaration.kind.toLowerCase();
  return normalized === expected;
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

function PackageSourcePane({ tile, focus, textRefs, row, onInspectFile, workingCodeForPath, proposalCodeForPath }: { tile: Tile; focus: FocusTarget | null; textRefs: React.MutableRefObject<Record<string, HTMLPreElement | null>>; row: InquiryRow; onInspectFile: (row: InquiryRow, tile: Tile, file: File) => void; workingCodeForPath: (packageDirectory: string, packageName: string, filePath: string) => FileEditState | undefined; proposalCodeForPath: (packageDirectory: string, packageName: string, filePath: string) => FileEditState | undefined }) {
  const files = tile.overview.files ?? [];
  return <div className="package-source-pane"><nav className="file-rail" aria-label="Package files"><div className="file-rail-title">FILES</div>{files.map((file) => <button type="button" className={tile.text.filename === file.path ? 'file-rail-item selected' : 'file-rail-item'} key={file.path} onClick={() => onInspectFile(row, tile, file)}><strong>{file.name}</strong><small>{file.path}</small></button>)}</nav><div className="package-source"><TextRepresentation tile={tile} focus={focus} textRefs={textRefs} editState={workingCodeForPath(tile.target.packagePath ?? '', tile.target.packageName ?? '', tile.text.filename ?? '')} alternateEditState={proposalCodeForPath(tile.target.packagePath ?? '', tile.target.packageName ?? '', tile.text.filename ?? '')} alternateLabel="Immutable proposal" formatRequest={0} /></div></div>;
}

function TargetButton({ label, detail, hasEdits = false, canOpenLeft, onOpen, onOpenLeft, onOpenColumn }: { label: string; detail: string; hasEdits?: boolean; canOpenLeft: boolean; onOpen: () => void; onOpenLeft: () => void; onOpenColumn: () => void }) {
  return <div className={`target-choice${hasEdits ? ' has-edits' : ''}`}><button type="button" className="target-button" onClick={onOpen}><strong>{label}</strong><small>{detail}</small>{hasEdits && <span className="target-edit-marker">AST edits below</span>}</button>{canOpenLeft && <button type="button" className="target-column-button" onClick={onOpenLeft}>Open left</button>}<button type="button" className="target-column-button" onClick={onOpenColumn}>+ column</button></div>;
}

function ReferenceButton({ reference, canOpenLeft, onOpen, onOpenLeft, onOpenColumn }: { reference: Reference; canOpenLeft: boolean; onOpen: () => void; onOpenLeft: () => void; onOpenColumn: () => void }) {
  return <div className="target-choice reference-choice"><button type="button" className="target-button" aria-label="Open reference in current tile" onClick={onOpen}><strong>{reference.filePath}</strong><small>{reference.kind} {reference.declaration} · line {reference.referenceLine ?? reference.line}</small></button>{canOpenLeft && <button type="button" className="target-column-button" aria-label="Open reference in left column" onClick={onOpenLeft}>Open left</button>}<button type="button" className="target-column-button" aria-label="Open reference in new column" onClick={onOpenColumn}>+ column</button></div>;
}
