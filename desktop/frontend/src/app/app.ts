import { Component, HostListener, signal } from '@angular/core';

import { ApplyEditSiblings, ApplyEditSubtree, ApplyEditView, ApplyEditWithOptions, ExplorePreviousCommitEdits, OpenDeclaration, OpenFile, OpenImportedDeclaration, OpenImportedFile, OpenPackage, OpenProgram, ProjectEdits, Reconcile, ReconcileGroup, ReconcileSelected, RemoveEdit, RemoveEditSiblings, RemoveEditSubtree, RemoveEditView } from '../../wailsjs/go/main/App';

interface EditRow {
  nodeId: string;
  nodeGlobalId?: string;
  sourceGlobalId?: string;
  parentId?: string;
  parentGlobalId?: string;
  kind: string;
  nodeKind: string;
  field?: string;
  position: number;
  value?: string;
}

interface EditView extends EditRow {
  index: number;
  depth: number;
  hasChildren: boolean;
  descendantCount: number;
}

interface ProjectedEdit {
  editIndex: number;
  depth: number;
  hasChildren: boolean;
  descendantCount: number;
}

interface ProgramSnapshot {
  path: string;
  source: string;
  target: string;
  working?: Record<string, unknown>;
  workingCode: string;
  renderDiagnostics?: string[];
  edits: EditRow[];
  status: string[];
  valid: boolean;
  diagnostics?: string[];
  candidates?: ProgramCandidate[];
  editViews?: ProjectedEdit[];
  exploration?: boolean;
  packages?: PackageSummary[];
  packageName?: string;
  files?: FileSummary[];
  packageDirectory?: string;
  fileName?: string;
  filePath?: string;
  declarations?: DeclarationSummary[];
  localImports?: string[];
  imports?: ImportSummary[];
  importedFileName?: string;
  importedSource?: string;
  importedName?: string;
  fileSource?: string;
  declarationName?: string;
  declarationSource?: string;
}

interface PackageSummary {
  name: string;
  directory: string;
  fileCount: number;
  files?: FileSummary[];
}

interface FileSummary {
  name: string;
  path: string;
  declarations?: DeclarationSummary[];
  samePackageReferences?: ReferenceSummary[];
}

interface ReferenceSummary {
  name: string;
  kind: string;
  packagePath: string;
  filePath: string;
  declaration: string;
  line: number;
}

interface DeclarationSummary {
  kind: string;
  name: string;
  receiver?: string;
  line: number;
  endLine: number;
  exported: boolean;
  parameters?: ParameterSummary[];
  results?: ParameterSummary[];
  type?: string;
  typeReferences?: ReferenceSummary[];
  children?: DeclarationSummary[];
}

interface ParameterSummary {
  name?: string;
  type: string;
}

interface ImportSummary {
  path: string;
  name: string;
  directory: string;
  files: FileSummary[];
  declarations: DeclarationSummary[];
}

interface DeclarationRow {
  declaration: DeclarationSummary;
  child: boolean;
}

interface PackageApiRow {
  file: FileSummary;
  declaration: DeclarationSummary;
  child: boolean;
}

function showDeclarationFor(declaration: DeclarationSummary, visibility: 'all' | 'exported' | 'unexported'): boolean {
  if (declaration.kind === 'struct' || declaration.kind === 'type') {
    return visibility === 'all' || (declaration.children ?? []).some((child) => visibility === 'exported' ? child.exported : !child.exported);
  }
  return visibility === 'all' || (visibility === 'exported' ? declaration.exported : !declaration.exported);
}

interface ProgramCandidate {
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
}

interface AstNode {
  id: string;
  globalId?: string;
  currentPath?: string;
  currentParentId?: string;
  currentField?: string;
  currentIndex?: number;
  kind: string;
  value?: string;
  field?: string;
  index?: number;
  children?: AstNode[];
}

interface StructuralIssue {
  editIndex: number;
  nodeId: string;
  nodeKind: string;
  nodeValue?: string;
  actualPath: string;
  actualParent: string;
  intendedParent: string;
  intendedField: string;
  intendedIndex: number;
  reason: string;
}

interface AstLocation {
  node: AstNode;
  parent?: AstNode;
  path: string;
  field?: string;
  index?: number;
}

interface OperationSummary {
  label: string;
  editIndex?: number;
  astChanged: boolean;
  sourceChanged: boolean;
}

interface LiftedShellOption {
  kind: string;
  label: string;
}

@Component({
  imports: [],
  selector: 'app-root',
  styleUrl: './app.css',
  templateUrl: './app.html',
})
export class App {
  protected readonly programPath = signal('../TestProgram');
  protected readonly snapshot = signal<ProgramSnapshot | null>(null);
  protected readonly error = signal('');
  protected readonly loading = signal(false);
  protected readonly expandedEdits = signal<Record<number, boolean>>({});
  protected readonly expandedPackages = signal<Record<string, boolean>>({});
  protected readonly expandedFiles = signal<Record<string, boolean>>({});
  protected readonly declarationVisibility = signal<'all' | 'exported' | 'unexported'>('all');
  protected readonly packageLens = signal<'files' | 'api'>('files');
  protected readonly selectedDeclarationLine = signal<number | null>(null);
  protected readonly apiSource = signal<ProgramSnapshot | null>(null);
  protected readonly importedSource = signal<ProgramSnapshot | null>(null);
  protected readonly importedPackage = signal<ImportSummary | null>(null);
  protected readonly importedFilePath = signal<string | null>(null);
  protected readonly importedDeclarationLine = signal<number | null>(null);
  protected readonly importedPackageLens = signal<'files' | 'api'>('files');
  protected readonly referencedUsageLines = signal<number[]>([]);
  protected readonly expandedImports = signal<Record<string, boolean>>({});
  protected readonly explorerSelection = signal(0);
  protected readonly editRows = signal<EditView[]>([]);
  protected readonly editView = signal<'ast' | 'lifted'>('lifted');
  protected readonly liftedShells = signal<Record<string, boolean>>({
    '*ast.BlockStmt': true,
    '*ast.DeclStmt': true,
    '*ast.ExprStmt': true,
    '*ast.Field': true,
    '*ast.FieldList': true,
    '*ast.ImportSpec': true,
  });
  protected readonly liftedShellOptions: LiftedShellOption[] = [
    { kind: '*ast.BlockStmt', label: 'BlockStmt bodies' },
    { kind: '*ast.ExprStmt', label: 'ExprStmt wrappers' },
    { kind: '*ast.DeclStmt', label: 'DeclStmt wrappers' },
    { kind: '*ast.ImportSpec', label: 'ImportSpec wrappers' },
    { kind: '*ast.FieldList', label: 'FieldList wrappers' },
    { kind: '*ast.Field', label: 'Field wrappers' },
  ];
  protected readonly selectedEditIndex = signal<number | null>(null);
  protected readonly inspectorTab = signal<'code' | 'ast'>('code');
  protected readonly astJson = signal('');
  protected readonly structuralIssues = signal<StructuralIssue[]>([]);
  protected readonly autoReconcile = signal(true);
  protected readonly selectedCandidates = signal<Record<string, boolean>>({});
  protected readonly lastOperation = signal<OperationSummary | null>(null);

  protected openProgram(): void {
    this.loading.set(true);
    this.error.set('');
    OpenProgram(this.programPath()).then((snapshot) => {
      const current = snapshot as ProgramSnapshot;
      this.snapshot.set(current);
      this.apiSource.set(null);
      this.expandedEdits.set({});
      this.expandedPackages.set({});
      this.expandedFiles.set({});
      this.explorerSelection.set(0);
      this.lastOperation.set(null);
      this.rebuildViews(current);
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected explorePreviousCommitEdits(): void {
    this.loading.set(true);
    this.error.set('');
    ExplorePreviousCommitEdits(this.programPath()).then((snapshot) => {
      const current = snapshot as ProgramSnapshot;
      this.snapshot.set(current);
      this.apiSource.set(null);
      this.expandedEdits.set({});
      this.expandedPackages.set({});
      this.expandedFiles.set({});
      this.explorerSelection.set(0);
      this.lastOperation.set(null);
      this.rebuildViews(current);
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected openPackage(pkg: PackageSummary): void {
    this.loading.set(true);
    this.error.set('');
    OpenPackage(this.programPath(), pkg.directory, pkg.name).then((snapshot) => {
      const current = snapshot as ProgramSnapshot;
      this.snapshot.set(current);
      this.apiSource.set(null);
      this.packageLens.set('files');
      this.explorerSelection.set(0);
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected backToPackages(): void {
    this.loading.set(true);
    this.error.set('');
    OpenProgram(this.programPath()).then((snapshot) => {
      const current = snapshot as ProgramSnapshot;
      this.snapshot.set(current);
      this.apiSource.set(null);
      this.packageLens.set('files');
      this.expandedPackages.set({});
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected togglePackage(pkg: PackageSummary, event: Event): void {
    event.stopPropagation();
    const key = this.packageKey(pkg);
    this.expandedPackages.update((expanded) => ({ ...expanded, [key]: !expanded[key] }));
  }

  protected packageKey(pkg: PackageSummary): string {
    return `${pkg.directory}:${pkg.name}`;
  }

  protected openFile(file: FileSummary): void {
    const current = this.snapshot();
    if (!current?.packageName || current.packageDirectory === undefined) {
      return;
    }
    this.loading.set(true);
    this.error.set('');
    OpenFile(this.programPath(), current.packageDirectory, current.packageName, file.path).then((snapshot) => {
      this.snapshot.set(snapshot as ProgramSnapshot);
      this.importedSource.set(null);
      this.importedFilePath.set(null);
      this.importedDeclarationLine.set(null);
      this.referencedUsageLines.set([]);
      this.importedPackage.set(null);
      this.declarationVisibility.set('all');
      this.explorerSelection.set(0);
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected openApiRow(row: PackageApiRow): void {
    this.loading.set(true);
    this.error.set('');
    OpenFile(this.programPath(), this.snapshot()?.packageDirectory ?? '', this.snapshot()?.packageName ?? '', row.file.path).then((snapshot) => {
      this.apiSource.set(snapshot as ProgramSnapshot);
      this.declarationVisibility.set('all');
      this.loading.set(false);
      this.selectDeclaration(row.declaration);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected openPackageFile(pkg: PackageSummary, file: FileSummary): void {
    this.loading.set(true);
    this.error.set('');
    OpenFile(this.programPath(), pkg.directory, pkg.name, file.path).then((snapshot) => {
      this.snapshot.set(snapshot as ProgramSnapshot);
      this.importedSource.set(null);
      this.importedFilePath.set(null);
      this.importedDeclarationLine.set(null);
      this.referencedUsageLines.set([]);
      this.importedPackage.set(null);
      this.declarationVisibility.set('all');
      this.explorerSelection.set(0);
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected openImportedDeclaration(imported: ImportSummary, declaration: DeclarationSummary): void {
    this.importedPackage.set(imported);
    this.importedFilePath.set(null);
    this.importedDeclarationLine.set(declaration.line);
    this.loading.set(true);
    this.error.set('');
    OpenImportedDeclaration(this.programPath(), imported.path, declaration.name, declaration.line).then((snapshot) => {
      this.importedSource.set(snapshot as ProgramSnapshot);
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected openImportedPackage(imported: ImportSummary): void {
    this.importedPackage.set(imported);
    this.importedSource.set(null);
    this.importedFilePath.set(null);
    this.importedDeclarationLine.set(null);
    this.importedPackageLens.set('files');
    this.referencedUsageLines.set([]);
  }

  protected openRelatedReference(reference: ReferenceSummary): void {
    const imported: ImportSummary = { path: reference.packagePath, name: reference.packagePath.split('/').pop() ?? reference.packagePath, directory: '', files: [], declarations: [] };
    this.importedPackage.set(imported);
    this.importedFilePath.set(reference.filePath);
    this.importedDeclarationLine.set(reference.line);
    this.openImportedFile(imported, { name: reference.filePath.split('/').pop() ?? reference.filePath, path: reference.filePath });
  }

  protected toggleImportedPackage(imported: ImportSummary, event: Event): void {
    event.stopPropagation();
    this.expandedImports.update((expanded) => ({ ...expanded, [imported.path]: !expanded[imported.path] }));
  }

  protected openImportedFile(imported: ImportSummary, file: FileSummary): void {
    this.importedPackage.set(imported);
    this.importedFilePath.set(file.path);
    this.importedDeclarationLine.set(null);
    this.loading.set(true);
    this.error.set('');
    OpenImportedFile(this.programPath(), imported.path, file.path).then((snapshot) => {
      this.importedPackage.set(imported);
      this.importedSource.set(snapshot as ProgramSnapshot);
      this.importedPackageLens.set('files');
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected selectImportedDeclaration(imported: ImportSummary, declaration: DeclarationSummary): void {
    this.importedPackage.set(imported);
    this.importedDeclarationLine.set(declaration.line);
    this.loading.set(true);
    this.error.set('');
    OpenImportedDeclaration(this.programPath(), imported.path, declaration.name, declaration.line).then((snapshot) => {
      this.importedPackage.set(imported);
      this.importedSource.set(snapshot as ProgramSnapshot);
      this.importedPackageLens.set('api');
      const alias = imported.path.split('/').pop() ?? imported.name;
      const pattern = new RegExp(`\\b${alias}\\.${declaration.name}\\b`);
      this.referencedUsageLines.set(this.sourceLines().flatMap((line, index) => pattern.test(line) ? [index + 1] : []));
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected importedApiRows(imported: ImportSummary): Array<{ declaration: DeclarationSummary; child: boolean }> {
    return imported.declarations.flatMap((declaration) => [
      { declaration, child: false },
      ...(declaration.children ?? []).filter((child) => child.exported).map((child) => ({ declaration: child, child: true })),
    ]);
  }

  protected sourceLineReferenced(line: number): boolean {
    return this.referencedUsageLines().includes(line);
  }

  protected hasExportedChild(declaration: DeclarationSummary): boolean {
    return (declaration.children ?? []).some((child) => child.exported);
  }

  protected backToFiles(): void {
    const current = this.snapshot();
    if (!current?.packageName || current.packageDirectory === undefined) {
      return;
    }
    this.loading.set(true);
    this.error.set('');
    OpenPackage(this.programPath(), current.packageDirectory, current.packageName).then((snapshot) => {
      this.snapshot.set(snapshot as ProgramSnapshot);
      this.expandedFiles.set({});
      this.explorerSelection.set(0);
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected openDeclaration(declaration: DeclarationSummary): void {
    const current = this.snapshot();
    if (!current?.packageName || current.packageDirectory === undefined || !current.filePath) {
      return;
    }
    this.loading.set(true);
    this.error.set('');
    OpenDeclaration(this.programPath(), current.packageDirectory, current.packageName, current.filePath, declaration.name, declaration.line).then((snapshot) => {
      this.snapshot.set(snapshot as ProgramSnapshot);
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected backToDeclarationList(): void {
    const current = this.snapshot();
    if (!current?.packageName || current.packageDirectory === undefined || !current.filePath) {
      return;
    }
    this.openFile({ name: current.fileName ?? current.filePath, path: current.filePath });
  }

  protected toggleFile(file: FileSummary, event: Event): void {
    event.stopPropagation();
    this.expandedFiles.update((expanded) => ({ ...expanded, [file.path]: !expanded[file.path] }));
  }

  protected declarationVisible(declaration: DeclarationSummary): boolean {
    const visibility = this.declarationVisibility();
    return visibility === 'all' || (visibility === 'exported' ? declaration.exported : !declaration.exported);
  }

  protected visibleChildren(declaration: DeclarationSummary): DeclarationSummary[] {
    return (declaration.children ?? []).filter((child) => this.declarationVisible(child));
  }

  protected showDeclaration(declaration: DeclarationSummary): boolean {
    return showDeclarationFor(declaration, this.declarationVisibility());
  }

  protected declarationSignature(declaration: DeclarationSummary): string {
    if (declaration.kind === 'function' || declaration.kind === 'method') {
      const parameters = (declaration.parameters ?? []).map((parameter) => parameter.name ? `${parameter.name} ${parameter.type}` : parameter.type).join(', ');
      const results = (declaration.results ?? []).map((result) => result.type).join(', ');
      return `(${parameters})${results ? ` -> ${results}` : ''}`;
    }
    return declaration.type ?? '';
  }

  protected currentFileReferences(): ReferenceSummary[] {
    const current = this.snapshot();
    return current?.files?.find((file) => file.path === current.filePath)?.samePackageReferences ?? [];
  }

  protected exportedDeclarations(file: FileSummary): DeclarationSummary[] {
    return (file.declarations ?? []).filter((declaration) => declaration.exported || declaration.children?.some((child) => child.exported));
  }

  protected exportedApiRows(files: FileSummary[] = this.snapshot()?.files ?? []): PackageApiRow[] {
    return files.flatMap((file) => (file.declarations ?? []).flatMap((declaration) => {
      if (!declaration.exported && !(declaration.children ?? []).some((child) => child.exported)) {
        return [];
      }
      return [
        { file, declaration, child: false },
        ...(declaration.children ?? []).filter((child) => child.exported).map((child) => ({ file, declaration: child, child: true })),
      ];
    }));
  }

  protected visibleDeclarationItems(declarations: DeclarationSummary[] = this.snapshot()?.declarations ?? []): DeclarationSummary[] {
    return declarations.flatMap((declaration) => showDeclarationFor(declaration, this.declarationVisibility()) ? [declaration, ...(declaration.children ?? []).filter((child) => this.declarationVisible(child))] : []);
  }

  protected declarationRows(): DeclarationRow[] {
    return (this.snapshot()?.declarations ?? []).flatMap((declaration) => {
      if (!this.showDeclaration(declaration)) {
        return [];
      }
      return [
        { declaration, child: false },
        ...this.visibleChildren(declaration).map((child) => ({ declaration: child, child: true })),
      ];
    });
  }

  protected declarationRowsFor(snapshot: ProgramSnapshot): DeclarationRow[] {
    return (snapshot.declarations ?? []).flatMap((declaration) => [
      { declaration, child: false },
      ...(declaration.children ?? []).map((child) => ({ declaration: child, child: true })),
    ]);
  }

  protected importedSourceLines(): string[] {
    const source = this.importedSource();
    return (source?.fileSource ?? source?.importedSource ?? '').split('\n');
  }

  protected sourceLines(): string[] {
    return (this.activeSource()?.fileSource ?? '').split('\n');
  }

  protected activeSource(): ProgramSnapshot | null {
    return this.apiSource() ?? this.snapshot();
  }

  protected sourceLineVisible(line: number): boolean {
    const declaration = (this.activeSource()?.declarations ?? []).flatMap((item) => [item, ...(item.children ?? [])]).find((item) => line >= item.line && line <= item.endLine);
    return !declaration || this.showDeclaration(declaration);
  }

  protected sourceLineSelected(line: number): boolean {
    const selected = this.selectedDeclarationLine();
    const declaration = (this.activeSource()?.declarations ?? []).flatMap((item) => [item, ...(item.children ?? [])]).find((item) => item.line === selected);
    return selected !== null && declaration !== undefined && line >= declaration.line && line <= declaration.endLine;
  }

  protected selectDeclaration(declaration: DeclarationSummary): void {
    this.selectedDeclarationLine.set(declaration.line);
    setTimeout(() => document.getElementById(`source-line-${declaration.line}`)?.scrollIntoView({ behavior: 'smooth', block: 'center' }), 0);
  }

  protected applyEdit(index: number): void {
    const operation = this.editView() === 'lifted'
      ? () => ApplyEditView(index, this.autoReconcile(), this.hiddenKinds())
      : () => ApplyEditWithOptions(index, this.autoReconcile());
    this.applyWithLiftedParent(index, operation, 'Apply edit');
  }

  protected applyEditSubtree(index: number): void {
    const operation = this.editView() === 'lifted'
      ? () => ApplyEditView(index, this.autoReconcile(), this.hiddenKinds()).then(() => ApplyEditSubtree(index))
      : () => ApplyEditSubtree(index);
    this.applyWithLiftedParent(index, operation, 'Apply edit subtree');
  }

  private applyWithLiftedParent(index: number, operation: () => Promise<unknown>, label: string): void {
    operation()
      .then((snapshot) => this.setSnapshot(snapshot as ProgramSnapshot, label, index))
      .catch((error: Error) => this.error.set(error.message));
  }

  protected toggleEditSiblings(index: number): void {
    const current = this.snapshot();
    if (!current) {
      return;
    }
    const edit = current.edits[index];
    const applied = edit !== undefined && current.edits.some((candidate, siblingIndex) => candidate.parentId === edit.parentId && current.status[siblingIndex] === 'applied');
    if (applied) {
      RemoveEditSiblings(index).then((snapshot) => this.setSnapshot(snapshot as ProgramSnapshot, 'Remove siblings', index))
        .catch((error: Error) => this.error.set(error.message));
    } else {
      const operation = this.editView() === 'lifted'
        ? () => ApplyEditView(index, this.autoReconcile(), this.hiddenKinds()).then(() => ApplyEditSiblings(index))
        : () => ApplyEditSiblings(index);
      this.applyWithLiftedParent(index, operation, 'Apply siblings');
    }
  }

  protected siblingsApplied(index: number): boolean {
    const current = this.snapshot();
    const edit = current?.edits[index];
    return current !== null && edit !== undefined && current.edits.some((candidate, siblingIndex) => candidate.parentId === edit.parentId && current.status[siblingIndex] === 'applied');
  }

  protected removeEdit(index: number): void {
    const operation = this.editView() === 'lifted'
      ? () => RemoveEditView(index, this.hiddenKinds())
      : () => RemoveEdit(index);
    this.removeWithLiftedParent(index, operation, 'Remove edit');
  }

  protected removeEditSubtree(index: number): void {
    this.removeWithLiftedParent(index, () => RemoveEditSubtree(index), 'Remove edit subtree');
  }

  private removeWithLiftedParent(index: number, operation: () => Promise<unknown>, label: string): void {
    operation().then((snapshot) => {
      this.setSnapshot(snapshot as ProgramSnapshot, label, index);
    }).catch((error: Error) => this.error.set(error.message));
  }

  protected reconcile(candidate: ProgramCandidate): void {
    Reconcile(candidate).then((snapshot) => this.setSnapshot(snapshot as ProgramSnapshot, 'Reconcile node'))
      .catch((error: Error) => this.error.set(error.message));
  }

  protected reconcileGroup(candidate: ProgramCandidate): void {
    ReconcileGroup(candidate.ancestorId).then((snapshot) => this.setSnapshot(snapshot as ProgramSnapshot, 'Reconcile group'))
      .catch((error: Error) => this.error.set(error.message));
  }

  protected candidateSelected(candidate: ProgramCandidate): boolean {
    return this.selectedCandidates()[candidate.nodeGlobalId] === true;
  }

  protected toggleCandidate(candidate: ProgramCandidate, selected: boolean): void {
    this.selectedCandidates.update((current) => ({ ...current, [candidate.nodeGlobalId]: selected }));
  }

  protected selectedCandidateCount(): number {
    return Object.values(this.selectedCandidates()).filter(Boolean).length;
  }

  protected reconcileSelected(): void {
    const selected = Object.entries(this.selectedCandidates()).filter(([, checked]) => checked).map(([globalId]) => globalId);
    if (selected.length === 0) {
      return;
    }
    ReconcileSelected(selected).then((snapshot) => this.setSnapshot(snapshot as ProgramSnapshot, 'Reconcile selected'))
      .catch((error: Error) => this.error.set(error.message));
  }

  protected candidatesForEdit(index: number): ProgramCandidate[] {
    const current = this.snapshot();
    const edit = current?.edits[index];
    if (!current || !edit) {
      return [];
    }
    return (current.candidates ?? []).filter((candidate) => candidate.nodeId === edit.nodeId);
  }

  protected toggleExpandedEdit(index: number): void {
    this.expandedEdits.update((expanded) => ({ ...expanded, [index]: expanded[index] !== true }));
    this.rebuildViews(this.snapshot());
  }

  protected expandAll(): void {
    const expanded: Record<number, boolean> = {};
    this.editRows().forEach((edit) => {
      if (edit.hasChildren) {
        expanded[edit.index] = true;
      }
    });
    this.expandedEdits.set(expanded);
    this.rebuildViews(this.snapshot());
  }

  protected collapseAll(): void {
    this.expandedEdits.set({});
    this.rebuildViews(this.snapshot());
  }

  protected selectEdit(index: number): void {
    this.selectedEditIndex.set(index);
  }

  protected isSelectedEdit(index: number): boolean {
    return this.selectedEditIndex() === index;
  }

  protected isLiftedShellEnabled(kind: string): boolean {
    return this.liftedShells()[kind] === true;
  }

  protected toggleLiftedShell(kind: string, enabled: boolean): void {
    this.liftedShells.update((current) => ({ ...current, [kind]: enabled }));
    this.refreshProjectedEdits();
  }

  protected setEditView(view: 'ast' | 'lifted'): void {
    this.editView.set(view);
    this.refreshProjectedEdits();
  }

  @HostListener('window:keydown', ['$event'])
  protected handleKeyboard(event: KeyboardEvent): void {
    const target = event.target as HTMLElement | null;
    if (target?.matches('input, textarea, select, button, [contenteditable="true"]')) {
      return;
    }
    const current = this.snapshot();
    if (current?.exploration) {
      this.handleExplorerKeyboard(event, current);
      return;
    }
    const rows = this.visibleEditRows();
    if (rows.length === 0) {
      return;
    }
    const selected = this.selectedEditIndex();
    const currentPosition = Math.max(0, rows.findIndex((row) => row.index === selected));
    if (event.key === 'j' || event.key === 'ArrowDown') {
      event.preventDefault();
      this.selectEdit(rows[Math.min(currentPosition + 1, rows.length - 1)].index);
      this.focusSelectedRow();
      return;
    }
    if (event.key === 'k' || event.key === 'ArrowUp') {
      event.preventDefault();
      this.selectEdit(rows[Math.max(currentPosition - 1, 0)].index);
      this.focusSelectedRow();
      return;
    }
    const row = rows[currentPosition];
    if (event.key === 'Enter') {
      event.preventDefault();
      if (event.shiftKey) {
        this.toggleEditSubtree(row.index);
      } else {
        this.toggleEdit(row.index);
      }
      return;
    }
    if (event.key.toLowerCase() === 's') {
      event.preventDefault();
      this.toggleEditSiblings(row.index);
      return;
    }
    if (event.shiftKey && event.key.toLowerCase() === 'l') {
      event.preventDefault();
      if (row.hasChildren) {
        this.expandedEdits.update((expanded) => ({ ...expanded, [row.index]: true }));
        this.rebuildViews(this.snapshot());
      }
      return;
    }
    if (event.shiftKey && event.key.toLowerCase() === 'h') {
      event.preventDefault();
      this.expandedEdits.update((expanded) => ({ ...expanded, [row.index]: false }));
      this.rebuildViews(this.snapshot());
    }
  }

  private handleExplorerKeyboard(event: KeyboardEvent, current: ProgramSnapshot): void {
    const isFile = Boolean(current.fileName);
    const isPackage = Boolean(current.packageName) && !isFile;
    const items = isFile ? this.visibleDeclarationItems(current.declarations) : isPackage ? (this.packageLens() === 'api' ? this.exportedApiRows(current.files) : current.files ?? []) : current.packages ?? [];
    if (items.length === 0) {
      return;
    }
    const selection = Math.min(this.explorerSelection(), items.length - 1);
    if (event.key === 'j' || event.key === 'ArrowDown') {
      event.preventDefault();
      this.explorerSelection.set(Math.min(selection + 1, items.length - 1));
      return;
    }
    if (event.key === 'k' || event.key === 'ArrowUp') {
      event.preventDefault();
      this.explorerSelection.set(Math.max(selection - 1, 0));
      return;
    }
    if (event.key === 'h' || event.key === 'Escape') {
      event.preventDefault();
      if (isFile) this.backToFiles();
      else if (isPackage) this.backToPackages();
      return;
    }
    if (event.key === 'Enter' || event.key === 'l') {
      event.preventDefault();
      if (isFile) this.selectDeclaration((items as DeclarationSummary[])[selection]);
      if (isPackage) {
        if (this.packageLens() === 'api') this.openApiRow((items as PackageApiRow[])[selection]);
        else this.openFile((items as FileSummary[])[selection]);
      }
      else this.openPackage((items as PackageSummary[])[selection]);
      return;
    }
    if (event.key === ' ') {
      event.preventDefault();
      if (isPackage) this.toggleFile((items as FileSummary[])[selection], event);
      else if (!isFile) this.togglePackage((items as PackageSummary[])[selection], event);
    }
  }

  private toggleEdit(index: number): void {
    if (this.snapshot()?.status[index] === 'applied') {
      this.removeEdit(index);
    } else {
      this.applyEdit(index);
    }
  }

  private toggleEditSubtree(index: number): void {
    if (this.snapshot()?.status[index] === 'applied') {
      this.removeEditSubtree(index);
    } else {
      this.applyEditSubtree(index);
    }
  }

  private focusSelectedRow(): void {
    const index = this.selectedEditIndex();
    if (index === null) {
      return;
    }
    setTimeout(() => document.querySelector(`[data-edit-index="${index}"]`)?.scrollIntoView({ block: 'nearest' }), 0);
  }

  protected visibleEditRows(): EditView[] {
    return this.editRows();
  }

  private hiddenKinds(): string[] {
    if (this.editView() === 'ast') {
      return [];
    }
    return Object.entries(this.liftedShells()).filter(([, enabled]) => enabled).map(([kind]) => kind);
  }

  private refreshProjectedEdits(): void {
    if (!this.snapshot()) {
      return;
    }
    ProjectEdits(this.hiddenKinds()).then((projected) => {
      const current = this.snapshot();
      if (!current) return;
      this.setProjectedViews(current, projected as ProjectedEdit[]);
    }).catch((error: Error) => this.error.set(error.message));
  }

  protected functionEditCount(edit: EditView): number {
    const current = this.snapshot();
    if (!current) {
      return 0;
    }
    return current.edits.filter((candidate) => candidate.nodeId === edit.nodeId || candidate.nodeId.startsWith(edit.nodeId + '.')).length;
  }

  protected parentKind(edit: EditRow): string {
    if (!edit.parentId) {
      return 'root';
    }
    return this.snapshot()?.edits.find((candidate) => candidate.nodeId === edit.parentId)?.nodeKind ?? 'parent';
  }

  protected expandToEdit(index: number): void {
    const current = this.snapshot();
    if (!current) {
      return;
    }
    const byId = new Map(current.edits.map((edit, editIndex) => [edit.nodeId, editIndex]));
    const expanded = { ...this.expandedEdits() };
    let editIndex: number | undefined = index;
    while (editIndex !== undefined) {
      const edit: EditRow | undefined = current.edits[editIndex];
      if (!edit) {
        break;
      }
      expanded[editIndex] = true;
      editIndex = edit.parentId ? byId.get(edit.parentId) : undefined;
    }
    this.expandedEdits.set(expanded);
    this.rebuildViews(current);
  }

  private setSnapshot(snapshot: ProgramSnapshot, label?: string, editIndex?: number): void {
    const previous = this.snapshot();
    if (label) {
      this.lastOperation.set({
        label,
        editIndex,
        astChanged: JSON.stringify(previous?.working ?? null) !== JSON.stringify(snapshot.working ?? null),
        sourceChanged: previous?.workingCode !== snapshot.workingCode,
      });
    }
    this.snapshot.set(snapshot);
    this.selectedCandidates.set({});
    this.rebuildViews(snapshot);
    this.refreshProjectedEdits();
  }

  private rebuildViews(snapshot: ProgramSnapshot | null): void {
    if (!snapshot) {
      this.editRows.set([]);
      this.astJson.set('');
      this.structuralIssues.set([]);
      return;
    }

    this.setProjectedViews(snapshot, snapshot.editViews ?? []);
    const visibleIndexes = this.visibleEditRows().map((edit) => edit.index);
    if (!visibleIndexes.includes(this.selectedEditIndex() ?? -1)) {
      this.selectedEditIndex.set(visibleIndexes[0] ?? null);
    }
    this.astJson.set(snapshot.working ? JSON.stringify(snapshot.working, null, 2) : '');
    this.structuralIssues.set(this.findStructuralIssues(snapshot));
  }

  private setProjectedViews(snapshot: ProgramSnapshot, projected: ProjectedEdit[]): void {
    const rows = projected.map((view) => ({
      ...snapshot.edits[view.editIndex],
      index: view.editIndex,
      depth: view.depth,
      hasChildren: view.hasChildren,
      descendantCount: view.descendantCount,
    }));
    this.editRows.set(rows);
    const visibleIndexes = rows.map((edit) => edit.index);
    if (!visibleIndexes.includes(this.selectedEditIndex() ?? -1)) {
      this.selectedEditIndex.set(visibleIndexes[0] ?? null);
    }
  }

  protected issuesForEdit(index: number): StructuralIssue[] {
    return this.structuralIssues().filter((issue) => issue.editIndex === index);
  }

  protected highlightedWorkingCode(): string {
    const code = this.snapshot()?.workingCode ?? '';
    const issues = this.structuralIssues().map((issue) => `${issue.nodeKind} is at ${issue.actualPath}; intended ${issue.intendedParent}.${issue.intendedField}[${issue.intendedIndex}]`);
    const title = issues.length > 0
      ? `Structural error: ${issues.join(' | ')}`
      : 'Structural error: this node is incomplete';
    return this.renderStructuralMarkers(code, title);
  }

  private renderStructuralMarkers(code: string, title: string): string {
    const marker = 'STRUCTURALERROR.';
    const markerStart = code.indexOf(marker);
    if (markerStart < 0) {
      return this.escapeHtml(code);
    }
    const nameStart = markerStart + marker.length;
    let nameEnd = nameStart;
    while (nameEnd < code.length && /[A-Za-z]/.test(code[nameEnd])) {
      nameEnd++;
    }
    const markerName = code.slice(markerStart, nameEnd);
    const escapedTitle = this.escapeHtml(`${title} (${markerName})`).replaceAll('"', '&quot;');
    const box = `<span class="source-structural-error" title="${escapedTitle}" aria-label="${escapedTitle}"></span>`;
    if (code[nameEnd] !== '(') {
      return this.escapeHtml(code.slice(0, markerStart)) + box + this.renderStructuralMarkers(code.slice(nameEnd), title);
    }
    const close = this.findClosingParenthesis(code, nameEnd);
    if (close < 0) {
      return this.escapeHtml(code.slice(0, markerStart)) + box + this.renderStructuralMarkers(code.slice(nameEnd + 1), title);
    }
    const inner = code.slice(nameEnd + 1, close);
    return this.escapeHtml(code.slice(0, markerStart)) + box + this.renderStructuralMarkers(inner, title) + this.renderStructuralMarkers(code.slice(close + 1), title);
  }

  private findClosingParenthesis(value: string, open: number): number {
    let depth = 0;
    let quote = '';
    let escaped = false;
    for (let index = open; index < value.length; index++) {
      const character = value[index];
      if (quote) {
        if (escaped) {
          escaped = false;
        } else if (character === '\\') {
          escaped = true;
        } else if (character === quote) {
          quote = '';
        }
        continue;
      }
      if (character === '"' || character === '`' || character === "'") {
        quote = character;
      } else if (character === '(') {
        depth++;
      } else if (character === ')' && --depth === 0) {
        return index;
      }
    }
    return -1;
  }

  private escapeHtml(value: string): string {
    return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
  }

  private findStructuralIssues(snapshot: ProgramSnapshot): StructuralIssue[] {
    const root = snapshot.working as AstNode | undefined;
    if (!root) {
      return [];
    }

    const locations: AstLocation[] = [];
    const walk = (node: AstNode, parent?: AstNode, path = 'root', field?: string, index?: number): void => {
      locations.push({ node, parent, path, field, index });
      const fieldIndexes = new Map<string, number>();
      node.children?.forEach((child, childIndex) => {
        const field = child.field || 'children';
        const currentIndex = fieldIndexes.get(field) ?? 0;
        fieldIndexes.set(field, currentIndex + 1);
        walk(child, node, `${path}.${field}[${currentIndex}]`, field, currentIndex);
      });
    };
    walk(root);

    const editByGlobalId = new Map<string, number>();
    const editById = new Map<string, number>();
    snapshot.edits.forEach((edit, index) => {
      if (edit.nodeGlobalId) {
        editByGlobalId.set(edit.nodeGlobalId, index);
      }
      editById.set(edit.nodeId, index);
    });
    const issues: StructuralIssue[] = [];

    const addIssue = (location: AstLocation, reason: string, intendedField = '(valid structure)'): void => {
      const editIndex = location.node.globalId && editByGlobalId.has(location.node.globalId)
        ? editByGlobalId.get(location.node.globalId)!
        : editById.get(location.node.id) ?? -1;
      issues.push({
        editIndex,
        nodeId: location.node.id,
        nodeKind: location.node.kind,
        nodeValue: location.node.value,
        actualPath: location.path,
        actualParent: location.parent?.id ?? '(root)',
        intendedParent: location.parent?.id ?? '(root)',
        intendedField,
        intendedIndex: location.index ?? 0,
        reason,
      });
    };

    locations.forEach((location) => {
      const node = location.node;
      const parent = location.parent;
      if (parent?.kind === '*ast.BlockStmt' && node.field !== 'List' && !(node.field === 'child' && this.isStatementKind(node.kind))) {
        addIssue(location, `${node.kind} is not a valid statement in this block`, 'List');
      }
      if (node.kind === '*ast.AssignStmt' && (!this.hasField(node, 'Lhs') || !this.hasField(node, 'Rhs'))) {
        addIssue(location, 'assignment is missing its left-hand or right-hand expression');
      }
      if (node.kind === '*ast.CallExpr' && !this.hasField(node, 'Fun')) {
        addIssue(location, 'function call is missing its function expression');
      }
      if (node.kind === '*ast.SelectorExpr' && (!this.hasField(node, 'X') || !this.hasField(node, 'Sel'))) {
        addIssue(location, 'selector is missing its receiver or selector');
      }
      if (node.kind === '*ast.IfStmt' && (!this.hasField(node, 'Cond') || !this.hasField(node, 'Body'))) {
        addIssue(location, 'if statement is missing its condition or body');
      }
      if (node.kind === '*ast.FuncLit' && (!this.hasField(node, 'Type') || !this.hasField(node, 'Body'))) {
        addIssue(location, 'function literal is missing its type or body');
      }
    });
    return issues;
  }

  private hasField(node: AstNode, field: string): boolean {
    return node.children?.some((child) => child.field === field) === true;
  }

  private isStatementKind(kind: string): boolean {
    return ['*ast.AssignStmt', '*ast.BlockStmt', '*ast.BranchStmt', '*ast.CaseClause', '*ast.CommClause', '*ast.DeclStmt', '*ast.DeferStmt', '*ast.EmptyStmt', '*ast.ExprStmt', '*ast.ForStmt', '*ast.GoStmt', '*ast.IfStmt', '*ast.IncDecStmt', '*ast.LabeledStmt', '*ast.RangeStmt', '*ast.ReturnStmt', '*ast.SelectStmt', '*ast.SendStmt', '*ast.SwitchStmt', '*ast.TypeSwitchStmt'].includes(kind);
  }
}
