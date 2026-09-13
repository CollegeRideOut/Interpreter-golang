import { Component, HostListener, signal } from '@angular/core';

import { ApplyEditSiblings, ApplyEditSubtree, ApplyEditWithOptions, OpenProgram, Reconcile, ReconcileGroup, ReconcileSelected, RemoveEdit, RemoveEditSiblings, RemoveEditSubtree } from '../../wailsjs/go/main/App';

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
      this.expandedEdits.set({});
      this.lastOperation.set(null);
      this.rebuildViews(current);
      this.loading.set(false);
    }).catch((error: Error) => {
      this.error.set(error.message);
      this.loading.set(false);
    });
  }

  protected applyEdit(index: number): void {
    this.applyWithLiftedParent(index, () => ApplyEditWithOptions(index, this.autoReconcile()), 'Apply edit');
  }

  protected applyEditSubtree(index: number): void {
    this.applyWithLiftedParent(index, () => ApplyEditSubtree(index), 'Apply edit subtree');
  }

  private applyWithLiftedParent(index: number, operation: () => Promise<unknown>, label: string): void {
    const current = this.snapshot();
    const edit = current?.edits[index];
    const ancestors = edit && this.editView() === 'lifted' ? this.liftedAncestorIndexes(index) : [];
    const prepareParents = ancestors.reduce((promise, parentIndex) => promise.then(() => {
      if (current?.status[parentIndex] === 'applied') {
        return;
      }
      return ApplyEditWithOptions(parentIndex, this.autoReconcile()).then(() => undefined);
    }), Promise.resolve());
    prepareParents.then(() => operation())
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
      this.applyWithLiftedParent(index, () => ApplyEditSiblings(index), 'Apply siblings');
    }
  }

  protected siblingsApplied(index: number): boolean {
    const current = this.snapshot();
    const edit = current?.edits[index];
    return current !== null && edit !== undefined && current.edits.some((candidate, siblingIndex) => candidate.parentId === edit.parentId && current.status[siblingIndex] === 'applied');
  }

  protected removeEdit(index: number): void {
    this.removeWithLiftedParent(index, () => RemoveEdit(index), 'Remove edit');
  }

  protected removeEditSubtree(index: number): void {
    this.removeWithLiftedParent(index, () => RemoveEditSubtree(index), 'Remove edit subtree');
  }

  private removeWithLiftedParent(index: number, operation: () => Promise<unknown>, label: string): void {
    const current = this.snapshot();
    const edit = current?.edits[index];
    const parentIndex = this.editView() === 'lifted' && edit?.parentId
      ? current?.edits.findIndex((candidate) => candidate.nodeId === edit.parentId && this.isLiftedShell(candidate))
      : -1;
    operation().then((snapshot) => {
      const next = snapshot as ProgramSnapshot;
      const emptyShellParent = parentIndex !== undefined && parentIndex >= 0 && next.status[parentIndex] === 'applied' && !this.shellHasAppliedChildren(next, parentIndex);
      if (!emptyShellParent) {
        this.setSnapshot(next, label, index);
        return;
      }
      return RemoveEdit(parentIndex).then((cleaned) => this.setSnapshot(cleaned as ProgramSnapshot, `${label} wrapper`, index));
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
  }

  @HostListener('window:keydown', ['$event'])
  protected handleKeyboard(event: KeyboardEvent): void {
    const target = event.target as HTMLElement | null;
    if (target?.matches('input, textarea, select, button, [contenteditable="true"]')) {
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
    if (this.editView() === 'ast') {
      return this.editRows();
    }

    const current = this.snapshot();
    if (!current) {
      return [];
    }
    const children = new Map<string, number[]>();
    current.edits.forEach((edit, index) => {
      if (edit.parentId) {
        children.set(edit.parentId, [...(children.get(edit.parentId) ?? []), index]);
      }
    });
    const knownNodes = new Set(current.edits.map((edit) => edit.nodeId));
    const rows: EditView[] = [];
    const append = (index: number, depth: number): void => {
      const edit = current.edits[index];
      const childIndexes = children.get(edit.nodeId) ?? [];
      if (this.isLiftedShell(edit)) {
        childIndexes.forEach((childIndex) => append(childIndex, depth));
        return;
      }
      rows.push({
        ...edit,
        index,
        depth,
        hasChildren: childIndexes.length > 0,
        descendantCount: this.countEditDescendants(edit.nodeId, children),
      });
      if (childIndexes.length > 0 && this.expandedEdits()[index] === true) {
        childIndexes.forEach((childIndex) => append(childIndex, depth + 1));
      }
    };
    current.edits.forEach((edit, index) => {
      if (!edit.parentId || !knownNodes.has(edit.parentId)) {
        append(index, 0);
      }
    });
    return rows;
  }

  private isLiftedShell(edit: EditRow): boolean {
    if (this.isLiftedShellEnabled(edit.nodeKind) && this.isLiftedShellNode(edit.nodeKind)) {
      return true;
    }
    return this.isLiftedShellEnabled(edit.nodeKind) && edit.nodeKind === '*ast.BlockStmt' && edit.field !== 'List';
  }

  private isLiftedShellNode(nodeKind: string): boolean {
    return nodeKind === '*ast.ExprStmt' || nodeKind === '*ast.DeclStmt' || nodeKind === '*ast.ImportSpec' || nodeKind === '*ast.FieldList' || nodeKind === '*ast.Field';
  }

  private liftedAncestorIndexes(index: number): number[] {
    const current = this.snapshot();
    if (!current) {
      return [];
    }
    const ancestors: number[] = [];
    let parentId = current.edits[index]?.parentId;
    while (parentId) {
      const parentIndex = current.edits.findIndex((edit) => edit.nodeId === parentId);
      if (parentIndex < 0) {
        break;
      }
      if (!this.isLiftedShell(current.edits[parentIndex])) {
        break;
      }
      ancestors.unshift(parentIndex);
      parentId = current.edits[parentIndex].parentId;
    }
    return ancestors;
  }

  private shellHasAppliedChildren(snapshot: ProgramSnapshot, parentIndex: number): boolean {
    const parentId = snapshot.edits[parentIndex].nodeId;
    return snapshot.edits.some((edit, index) => edit.parentId === parentId && (snapshot.status[index] === 'applied' || snapshot.status[index] === 'prepared'));
  }

  private countEditDescendants(nodeId: string, children: Map<string, number[]>): number {
    return (children.get(nodeId) ?? []).reduce((total, childIndex) => {
      const child = this.snapshot()?.edits[childIndex];
      return total + 1 + (child ? this.countEditDescendants(child.nodeId, children) : 0);
    }, 0);
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
  }

  private rebuildViews(snapshot: ProgramSnapshot | null): void {
    if (!snapshot) {
      this.editRows.set([]);
      this.astJson.set('');
      this.structuralIssues.set([]);
      return;
    }

    const children = new Map<string, number[]>();
    snapshot.edits.forEach((edit, index) => {
      if (edit.parentId) {
        children.set(edit.parentId, [...(children.get(edit.parentId) ?? []), index]);
      }
    });
    const knownNodes = new Set(snapshot.edits.map((edit) => edit.nodeId));
    const countDescendants = (nodeId: string): number => {
      const descendants = children.get(nodeId) ?? [];
      return descendants.reduce((total, childIndex) => total + 1 + countDescendants(snapshot.edits[childIndex].nodeId), 0);
    };
    const rows: EditView[] = [];
    const appendEdit = (index: number, depth: number): void => {
      const edit = snapshot.edits[index];
      const childIndexes = children.get(edit.nodeId) ?? [];
      rows.push({ ...edit, index, depth, hasChildren: childIndexes.length > 0, descendantCount: countDescendants(edit.nodeId) });
      if (childIndexes.length > 0 && this.expandedEdits()[index] === true) {
        childIndexes.forEach((childIndex) => appendEdit(childIndex, depth + 1));
      }
    };
    snapshot.edits.forEach((edit, index) => {
      if (!edit.parentId || !knownNodes.has(edit.parentId)) {
        appendEdit(index, 0);
      }
    });
    this.editRows.set(rows);
    const visibleIndexes = this.visibleEditRows().map((edit) => edit.index);
    if (!visibleIndexes.includes(this.selectedEditIndex() ?? -1)) {
      this.selectedEditIndex.set(visibleIndexes[0] ?? null);
    }
    this.astJson.set(snapshot.working ? JSON.stringify(snapshot.working, null, 2) : '');
    this.structuralIssues.set(this.findStructuralIssues(snapshot));
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
