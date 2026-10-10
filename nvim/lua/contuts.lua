local M = {}
local clear_breakpoints
local launch_debug_targets

M.config = {
  command = "contuts",
  directory = nil,
  base = "HEAD~1",
  compare = "working-tree",
  project_file = ".contuts.json",
  env_file = ".contuts.env",
}

M.source_window = nil
M.data = nil
M.target_jobs = {}
M.breakpoint_buffers = {}
M.active_target = nil
M.active_targets = {}
M.last_plan = nil
M.coverage = {}
M.experiment = nil
M.project = {
  comparison = {},
  targets = {},
}
M.namespace = vim.api.nvim_create_namespace("contuts")

local function project_directory()
  if M.config.directory then
    return M.config.directory
  end
  return vim.fs.root(0, { M.config.project_file, ".git", "package.json" }) or vim.fn.getcwd()
end

local function load_project_config()
  local directory = project_directory()
  local config_path = directory .. "/" .. M.config.project_file
  if vim.fn.filereadable(config_path) == 1 then
    local ok, decoded = pcall(vim.json.decode, table.concat(vim.fn.readfile(config_path), "\n"))
    if ok and type(decoded) == "table" then
      M.project = vim.tbl_deep_extend("force", M.project, decoded)
    else
      vim.notify("contuts: invalid " .. M.config.project_file, vim.log.levels.ERROR)
    end
  end

  local env_path = directory .. "/" .. M.config.env_file
  if vim.fn.filereadable(env_path) == 1 then
    for _, line in ipairs(vim.fn.readfile(env_path)) do
      local key, value = line:match("^%s*([%w_]+)%s*=%s*(.-)%s*$")
      if key and value and not key:match("^#") then
        value = value:gsub("^['\"]", ""):gsub("['\"]$", "")
        vim.env[key] = value
      end
    end
  end
end

local function target_definitions()
  if type(M.project.targets) == "table" and #M.project.targets > 0 then
    return M.project.targets
  end

  -- Keep existing projects working while they move to the composable format.
  local targets = {}
  if M.project.backend and M.project.backend.entry then
    table.insert(targets, {
      name = "backend",
      kind = "node",
      entry = M.project.backend.entry,
    })
  end
  if M.project.frontend and M.project.frontend.url then
    table.insert(targets, {
      name = "frontend",
      kind = "browser",
      command = M.project.frontend.command,
      url = M.project.frontend.url,
    })
  end
  return targets
end

local function target_names()
  local names = {}
  for _, target in ipairs(target_definitions()) do
    table.insert(names, target.name or target.id or target.kind or "target")
  end
  return names
end

local function target_name(target)
  return target.name or target.id or target.kind or "target"
end

local function selected_target_definitions(names)
  local wanted = {}
  for _, name in ipairs(names or {}) do
    wanted[name] = true
  end
  local selected = {}
  for _, target in ipairs(target_definitions()) do
    if next(wanted) == nil or wanted[target_name(target)] then
      table.insert(selected, target)
    end
  end
  return selected
end

local function display_name(edit)
  local value = edit.value and (" " .. edit.value:gsub("[\r\n]+", " ")) or ""
  local location = edit.startLine and (" line " .. edit.startLine) or ""
  return string.format("%s %s%s%s", edit.kind, edit.nodeKind, value, location)
end

local function location_for_current_line()
  local locations = vim.b.contuts_locations
  if type(locations) ~= "table" then
    return nil
  end
  local location = locations[vim.fn.line(".")]
  if type(location) ~= "table" then
    return nil
  end
  return location
end

local function has_visible_ancestor(edit, visible)
  for _, ancestor in ipairs(edit.ancestorIds or {}) do
    if visible[ancestor] then
      return true
    end
  end
  return false
end

local function changed_lines(buffer, base, current)
  vim.api.nvim_buf_clear_namespace(buffer, M.namespace, 0, -1)
  local hunks = vim.diff(base, current, { result_type = "indices", algorithm = "histogram" }) or {}
  for _, hunk in ipairs(hunks) do
    local start_line = hunk[3]
    local new_count = hunk[4]
    local old_count = hunk[2]
    local group = old_count == 0 and "ContutsAdded" or (new_count == 0 and "ContutsRemoved" or "ContutsChanged")
    if new_count == 0 then
      vim.api.nvim_buf_set_extmark(buffer, M.namespace, math.max(0, start_line - 1), 0, {
        virt_text = {{ "- removed from current", "ContutsRemoved" }},
        virt_text_pos = "eol",
      })
    else
      for line = start_line, start_line + new_count - 1 do
        vim.api.nvim_buf_set_extmark(buffer, M.namespace, line - 1, 0, {
          line_hl_group = group,
        })
      end
    end
  end
end

local function highlight_file(path)
  local directory = project_directory()
  local buffer = vim.api.nvim_get_current_buf()
  local current = table.concat(vim.api.nvim_buf_get_lines(buffer, 0, -1, false), "\n")
  vim.system({ "git", "-C", directory, "show", M.config.base .. ":" .. path }, { text = true }, function(result)
    vim.schedule(function()
      local base = result.code == 0 and result.stdout or ""
      changed_lines(buffer, base, current)
    end)
  end)
end

function M.open_file()
  local location = location_for_current_line()
  if not location then
    return
  end
  local directory = project_directory()
  if M.source_window and vim.api.nvim_win_is_valid(M.source_window) then
    vim.api.nvim_set_current_win(M.source_window)
  end
  vim.cmd("edit " .. vim.fn.fnameescape(directory .. "/" .. location.path))
  vim.api.nvim_win_set_cursor(0, { math.max(1, location.line), 0 })
  highlight_file(location.path)
end

function M.open_compare()
  local location = location_for_current_line()
  if not location then
    return
  end
  local directory = project_directory()
  vim.system({ "git", "-C", directory, "show", M.config.base .. ":" .. location.path }, { text = true }, function(result)
    vim.schedule(function()
      local base = result.code == 0 and result.stdout or ""
      if M.source_window and vim.api.nvim_win_is_valid(M.source_window) then
        vim.api.nvim_set_current_win(M.source_window)
      end
      vim.cmd("edit " .. vim.fn.fnameescape(directory .. "/" .. location.path))
      local current_window = vim.api.nvim_get_current_win()
      local filetype = vim.bo.filetype
      vim.cmd("leftabove vsplit")
      local base_window = vim.api.nvim_get_current_win()
      local base_buffer = vim.api.nvim_create_buf(false, true)
      vim.api.nvim_win_set_buf(base_window, base_buffer)
      vim.api.nvim_buf_set_name(base_buffer, "contuts://base/" .. location.path)
      vim.bo[base_buffer].buftype = "nofile"
      vim.bo[base_buffer].bufhidden = "wipe"
      vim.bo[base_buffer].filetype = filetype
      vim.bo[base_buffer].modifiable = true
      vim.api.nvim_buf_set_lines(base_buffer, 0, -1, false, vim.split(base, "\n", { plain = true }))
      vim.bo[base_buffer].modifiable = false
      vim.bo[base_buffer].readonly = true
      vim.cmd("diffthis")
      vim.api.nvim_set_current_win(current_window)
      vim.cmd("diffthis")
      vim.keymap.set("n", "q", "<cmd>diffoff | close<cr>", { buffer = base_buffer, silent = true })
    end)
  end)
end

function M.render(data)
  local buffer = vim.api.nvim_get_current_buf()
  M.data = data
  local lines = {
    string.format("CONTUTS  %s -> %s", data.base, data.compare),
    "Press <CR> to open, o for before/after, r to refresh, q to close.",
    "",
  }
  local locations = {}
  for _, file in ipairs(data.files or {}) do
    table.insert(lines, file.path)
    locations[#lines] = { path = file.path, line = 1 }
    local visible = {}
    for _, edit in ipairs(file.edits or {}) do
      if not edit.hidden then
        visible[edit.nodeId] = true
      end
    end
    if file.added then
      table.insert(lines, "  [new file]")
      locations[#lines] = { path = file.path, line = 1 }
    elseif file.deleted then
      table.insert(lines, "  [deleted file]")
      locations[#lines] = { path = file.path, line = 1 }
    else
      for _, edit in ipairs(file.edits or {}) do
        if not edit.hidden and not has_visible_ancestor(edit, visible) then
          table.insert(lines, "  " .. display_name(edit))
          locations[#lines] = { path = file.path, line = edit.startLine or 1 }
        end
      end
    end
    table.insert(lines, "")
  end
  vim.bo[buffer].modifiable = true
  vim.api.nvim_buf_set_lines(buffer, 0, -1, false, lines)
  vim.bo[buffer].modifiable = false
  vim.bo[buffer].buftype = "nofile"
  vim.bo[buffer].filetype = "contuts"
  vim.bo[buffer].bufhidden = "wipe"
  vim.b.contuts_locations = locations
  vim.wo.number = false
  vim.wo.relativenumber = false
  vim.wo.cursorline = true
  vim.wo.foldmethod = "indent"
  vim.wo.foldenable = true
  vim.wo.foldlevel = 1
  vim.keymap.set("n", "<CR>", M.open_file, { buffer = buffer, silent = true })
  vim.keymap.set("n", "o", M.open_compare, { buffer = buffer, silent = true })
  vim.keymap.set("n", "r", function()
    vim.cmd("ContutsRefresh")
  end, { buffer = buffer, silent = true })
  vim.keymap.set("n", "q", "<cmd>close<cr>", { buffer = buffer, silent = true })
end

local function refresh_overview()
  local directory = project_directory()
  vim.system({ M.config.command, "--nvim-json", directory, M.config.base, M.config.compare }, { text = true }, function(result)
    vim.schedule(function()
      if result.code ~= 0 then
        vim.notify("contuts: " .. (result.stderr or "unable to load edits"), vim.log.levels.ERROR)
        return
      end
      local ok, data = pcall(vim.json.decode, result.stdout)
      if not ok then
        vim.notify("contuts: invalid JSON response", vim.log.levels.ERROR)
        return
      end
      M.render(data)
    end)
  end)
end

clear_breakpoints = function()
  local breakpoints = require("dap.breakpoints")
  breakpoints.clear()
  M.breakpoint_buffers = {}
end

local function coverage_key(path, line)
  return path .. ":" .. tostring(line)
end

local function shell_quote(value)
  return "'" .. tostring(value):gsub("'", "'\\''") .. "'"
end

local function expand_experiment_command(command)
  if not M.experiment then
    return command
  end
  return command
    :gsub("{experiment}", shell_quote(M.experiment.directory))
    :gsub("{id}", shell_quote(M.experiment.id))
end

local function experiment_id()
  return os.date("%Y%m%d-%H%M%S") .. "-" .. (M.config.base or "base"):gsub("[^%w%-]", "-")
end

local function write_experiment_metadata()
  if not M.experiment then
    return
  end
  local metadata = {
    id = M.experiment.id,
    directory = M.experiment.directory,
    base = M.config.base,
    current = M.config.compare,
    targets = M.active_targets,
    branch = vim.fn.system({ "git", "-C", project_directory(), "branch", "--show-current" }):gsub("%s+$", ""),
    commit = vim.fn.system({ "git", "-C", project_directory(), "rev-parse", "HEAD" }):gsub("%s+$", ""),
    parent = M.experiment.parent,
    envFiles = (M.project.experiment or {}).envFiles or {},
    envFilesSaved = (M.project.experiment or {}).saveEnvFiles == true,
  }
  vim.fn.writefile({ vim.json.encode(metadata) }, M.experiment.directory .. "/metadata.json")
end

local function save_breakpoints()
  if not M.experiment then
    return
  end
  local values = {}
  for buffer, breakpoints in pairs(require("dap.breakpoints").get()) do
    local path = vim.api.nvim_buf_get_name(buffer)
    local directory = project_directory():gsub("/+$", "")
    local normalized = path:gsub("^file://", "")
    local relative = normalized:sub(1, #directory + 1) == directory .. "/"
      and normalized:sub(#directory + 2)
      or normalized
    for _, breakpoint in ipairs(breakpoints) do
      table.insert(values, { path = relative, line = breakpoint.line })
    end
  end
  vim.fn.writefile({ vim.json.encode(values) }, M.experiment.directory .. "/breakpoints.json")
end

local function append_experiment_event(event)
  if not M.experiment then
    return
  end
  local path = M.experiment.directory .. "/dap-events.jsonl"
  local file = io.open(path, "a")
  if file then
    file:write(vim.json.encode(event) .. "\n")
    file:close()
  end
end

local function prepare_experiment(callback)
  local experiment = M.project.experiment or {}
  local id = experiment.id or experiment_id()
  local root = experiment.directory or ".contuts/experiments"
  if root:sub(1, 1) ~= "/" then
    root = project_directory() .. "/" .. root
  end
  M.experiment = { id = id, directory = root .. "/" .. id }
  vim.fn.mkdir(M.experiment.directory, "p")
  if experiment.saveEnvFiles then
    local env_directory = M.experiment.directory .. "/env"
    vim.fn.mkdir(env_directory, "p")
    for _, env_file in ipairs(experiment.envFiles or {}) do
      local source = project_directory() .. "/" .. env_file
      local name = vim.fn.fnamemodify(env_file, ":t")
      if vim.fn.filereadable(source) == 1 then
        vim.fn.writefile(vim.fn.readfile(source), env_directory .. "/" .. name)
      end
    end
  end
  write_experiment_metadata()

  local database = experiment.database or {}
  if not database.enabled or not database.snapshotCommand then
    callback()
    return
  end

  local command = string.format(
    "CONTUTS_EXPERIMENT_ID=%s CONTUTS_EXPERIMENT_DIR=%s %s",
    shell_quote(id),
    shell_quote(M.experiment.directory),
    expand_experiment_command(database.snapshotCommand)
  )
  vim.notify("Contuts: saving database snapshot...", vim.log.levels.INFO)
  vim.system({ "sh", "-lc", command }, { cwd = project_directory(), text = true }, function(result)
    vim.schedule(function()
      if result.code ~= 0 then
        vim.notify("contuts: database snapshot failed: " .. (result.stderr or "unknown error"), vim.log.levels.ERROR)
        return
      end
      vim.notify("Contuts: database snapshot saved", vim.log.levels.INFO)
      callback()
    end)
  end)
end

local function read_json_file(path)
  if vim.fn.filereadable(path) ~= 1 then
    return nil
  end
  local ok, value = pcall(vim.json.decode, table.concat(vim.fn.readfile(path), "\n"))
  return ok and value or nil
end

local function load_saved_environment(directory, metadata)
  if not metadata.envFilesSaved then
    return
  end
  for _, env_file in ipairs(metadata.envFiles or {}) do
    local name = vim.fn.fnamemodify(env_file, ":t")
    local path = directory .. "/env/" .. name
    if vim.fn.filereadable(path) == 1 then
      for _, line in ipairs(vim.fn.readfile(path)) do
        local key, value = line:match("^%s*([%w_]+)%s*=%s*(.-)%s*$")
        if key and value and not key:match("^#") then
          vim.env[key] = value:gsub("^['\"]", ""):gsub("['\"]$", "")
        end
      end
    end
  end
end

local function load_saved_breakpoints(directory)
  local values = read_json_file(directory .. "/breakpoints.json") or {}
  local breakpoints = require("dap.breakpoints")
  clear_breakpoints()
  M.coverage = {}
  for _, value in ipairs(values) do
    local path = project_directory() .. "/" .. value.path
    local buffer = vim.fn.bufadd(path)
    if not vim.api.nvim_buf_is_loaded(buffer) then
      vim.bo[buffer].swapfile = false
      vim.fn.bufload(buffer)
    end
    breakpoints.set({}, buffer, value.line)
    M.coverage[coverage_key(value.path, value.line)] = {
      path = value.path,
      line = value.line,
      nodeKind = "saved",
      hitCount = 0,
      reached = false,
    }
  end
  M.breakpoint_buffers = {}
end

function M.rerun_experiment(directory)
  local metadata = read_json_file(directory .. "/metadata.json")
  if not metadata then
    vim.notify("contuts: experiment metadata is missing or invalid", vim.log.levels.ERROR)
    return
  end
  local id = os.date("%Y%m%d-%H%M%S") .. "-rerun"
  M.config.base = metadata.base or M.config.base
  M.config.compare = metadata.current or M.config.compare
  M.active_targets = metadata.targets or target_names()
  M.active_target = true
  M.experiment = {
    id = id,
    directory = directory .. "/reruns/" .. id,
    parent = directory,
  }
  vim.fn.mkdir(M.experiment.directory, "p")
  load_saved_environment(directory, metadata)
  load_saved_breakpoints(directory)
  write_experiment_metadata()

  local restore = ((M.project.experiment or {}).database or {}).restoreCommand
  if not restore then
    launch_debug_targets(M.active_targets)
    return
  end
  local snapshot = directory .. "/database.dump"
  local command = expand_experiment_command(restore):gsub("{snapshot}", shell_quote(snapshot))
  vim.notify("Contuts: restoring experiment database...", vim.log.levels.INFO)
  vim.system({ "sh", "-lc", command }, { cwd = project_directory(), text = true }, function(result)
    vim.schedule(function()
      if result.code ~= 0 then
        vim.notify("contuts: experiment database restore failed: " .. (result.stderr or "unknown error"), vim.log.levels.ERROR)
        return
      end
      launch_debug_targets(M.active_targets)
    end)
  end)
end

local function select_experiment(callback)
  local root = project_directory() .. "/.contuts/experiments"
  local choices = {}
  for _, metadata_path in ipairs(vim.fn.globpath(root, "*/metadata.json", false, true)) do
    local directory = vim.fn.fnamemodify(metadata_path, ":h")
    local metadata = read_json_file(metadata_path)
    if metadata then
      table.insert(choices, { directory = directory, label = string.format("%s  (%s)", metadata.id or vim.fn.fnamemodify(directory, ":t"), metadata.commit or "unknown commit") })
    end
  end
  if #choices == 0 then
    vim.notify("contuts: no saved experiments found", vim.log.levels.WARN)
    return
  end
  vim.ui.select(choices, {
    prompt = "Contuts rerun experiment",
    format_item = function(item)
      return item.label
    end,
  }, function(choice)
    if choice then
      callback(choice.directory)
    end
  end)
end

local function is_semantic_node(node_kind)
  return node_kind == "typescript:call_expression"
    or node_kind == "typescript:expression_statement"
    or node_kind == "typescript:return_statement"
    or node_kind == "typescript:if_statement"
    or node_kind == "typescript:switch_statement"
    or node_kind == "typescript:for_statement"
    or node_kind == "typescript:while_statement"
    or node_kind == "typescript:throw_statement"
    or node_kind == "typescript:assignment_expression"
    or node_kind == "typescript:pair"
    or node_kind == "typescript:variable_declarator"
    or node_kind == "typescript:lexical_declaration"
    or node_kind == "typescript:function_declaration"
    or node_kind == "typescript:method_definition"
end

local function apply_ast_breakpoints(data)
  local breakpoints = require("dap.breakpoints")
  clear_breakpoints()
  M.coverage = {}
  local directory = project_directory()
  for _, file in ipairs(data.files or {}) do
    if file.error == nil and file.added ~= true and file.deleted ~= true then
      local extension = file.path:match("%.([^.]+)$")
      if extension == "ts" or extension == "tsx" or extension == "js" or extension == "jsx" then
        local path = directory .. "/" .. file.path
        local buffer = vim.fn.bufadd(path)
        if not vim.api.nvim_buf_is_loaded(buffer) then
          vim.bo[buffer].swapfile = false
          vim.fn.bufload(buffer)
        end
        local seen = {}
        for _, edit in ipairs(file.edits or {}) do
          local line = tonumber(edit.startLine)
          if not edit.hidden
            and is_semantic_node(edit.nodeKind)
            and line
            and line > 0
            and not seen[line]
          then
            breakpoints.set({}, buffer, line)
            local key = coverage_key(file.path, line)
            M.coverage[key] = {
              path = file.path,
              line = line,
              nodeKind = edit.nodeKind,
              nodeId = edit.nodeId,
              hitCount = 0,
              reached = false,
            }
            seen[line] = true
          end
        end
        M.breakpoint_buffers[buffer] = true
      end
    end
  end
end

local function relative_source_path(path)
  local directory = project_directory():gsub("/+$", "")
  local normalized = path:gsub("^file://", "")
  if normalized:sub(1, #directory + 1) == directory .. "/" then
    return normalized:sub(#directory + 2)
  end
  return normalized
end

function M.record_stop(session, body)
  local frame = session and session.current_frame
  if not frame or not frame.source or not frame.source.path or not frame.line then
    return
  end
  local path = relative_source_path(frame.source.path)
  local target = M.coverage[coverage_key(path, frame.line)]
  if not target then
    return
  end
  target.hitCount = target.hitCount + 1
  target.reached = true
  target.lastReason = body and body.reason or "stopped"
  append_experiment_event({
    type = "stopped",
    timestamp = os.date("!%Y-%m-%dT%H:%M:%SZ"),
    reason = target.lastReason,
    path = target.path,
    line = target.line,
    hitCount = target.hitCount,
    session = session.config and session.config.name or nil,
  })
  vim.notify(
    string.format("Contuts reached changed code: %s:%d (%dx)", target.path, target.line, target.hitCount),
    vim.log.levels.INFO
  )
end

local function coverage_report()
  local targets = {}
  local reached = 0
  for _, target in pairs(M.coverage) do
    table.insert(targets, target)
    if target.reached then
      reached = reached + 1
    end
  end
  table.sort(targets, function(left, right)
    if left.path == right.path then
      return left.line < right.line
    end
    return left.path < right.path
  end)
  local lines = {
    "CONTUTS CHANGE COVERAGE",
    string.format("Reached: %d / %d", reached, #targets),
    "",
  }
  for _, target in ipairs(targets) do
    local status = target.reached and "reached" or "not reached"
    table.insert(lines, string.format("[%s] %s:%d  %dx  %s", target.reached and "x" or " ", target.path, target.line, target.hitCount, status))
  end
  if #targets == 0 then
    table.insert(lines, "No generated changed-code breakpoints are active.")
  end
  return lines
end

function M.open_coverage()
  vim.cmd("botright new")
  local buffer = vim.api.nvim_get_current_buf()
  vim.api.nvim_buf_set_name(buffer, "contuts://coverage")
  vim.bo[buffer].buftype = "nofile"
  vim.bo[buffer].bufhidden = "wipe"
  vim.bo[buffer].swapfile = false
  vim.bo[buffer].modifiable = true
  vim.api.nvim_buf_set_lines(buffer, 0, -1, false, coverage_report())
  vim.bo[buffer].modifiable = false
  vim.bo[buffer].filetype = "contuts"
  vim.keymap.set("n", "q", "<cmd>close<cr>", { buffer = buffer, silent = true })
end

local function focus_first_changed_source(data)
  local directory = project_directory()
  for _, file in ipairs(data.files or {}) do
    local extension = file.path:match("%.([^.]+)$")
    if file.error == nil and file.deleted ~= true and (extension == "ts" or extension == "tsx" or extension == "js" or extension == "jsx") then
      local line = nil
      for _, edit in ipairs(file.edits or {}) do
        if not edit.hidden and edit.startLine and edit.startLine > 0 then
          line = edit.startLine
          break
        end
      end
      if line then
        vim.cmd("edit " .. vim.fn.fnameescape(directory .. "/" .. file.path))
        vim.api.nvim_win_set_cursor(0, { line, 0 })
        return
      end
    end
  end
end

local function prepare_debug_breakpoints(callback)
  local directory = project_directory()
  vim.system({ M.config.command, "--nvim-json", directory, M.config.base, M.config.compare }, { text = true }, function(result)
    vim.schedule(function()
      if result.code ~= 0 then
        vim.notify("contuts: unable to create breakpoint plan: " .. (result.stderr or "unknown error"), vim.log.levels.ERROR)
        return
      end
      local ok, data = pcall(vim.json.decode, result.stdout)
      if not ok then
        vim.notify("contuts: invalid breakpoint plan", vim.log.levels.ERROR)
        return
      end
      apply_ast_breakpoints(data)
      M.last_plan = data
      focus_first_changed_source(data)
      save_breakpoints()
      callback()
    end)
  end)
end

local function stop_target_jobs(allow_recorder_to_finish)
  local browser_key = "__contuts_experiment_browser"
  local browser_job = M.target_jobs[browser_key]
  if M.experiment and M.experiment.browser_started and browser_job then
    local experiment_directory = M.experiment.directory
    vim.fn.writefile({}, experiment_directory .. "/STOP")
    M.target_jobs[browser_key] = nil
    local pid_path = experiment_directory .. "/browser/PID"
    if vim.fn.filereadable(pid_path) == 1 then
      local pid = vim.fn.readfile(pid_path)[1]
      if pid and pid:match("^%d+$") then
        -- Signal the recorder before waiting or exiting so Playwright closes
        -- its context while Chromium is still alive.
        vim.fn.system({ "kill", "-TERM", pid })
      end
    end
    -- Let Playwright close the context first so HAR and trace files flush.
    -- This must be synchronous during VimLeavePre: deferred callbacks do not
    -- reliably run after Neovim begins exiting.
    local status = vim.fn.jobwait({ browser_job }, 10000)[1]
    if status == -1 then
      if vim.fn.filereadable(pid_path) == 1 then
        local pid = vim.fn.readfile(pid_path)[1]
        if pid and pid:match("^%d+$") then
          -- The recorder owns Chromium. Signal the recorder, not the browser,
          -- so its Playwright handler can flush the context before closing it.
          vim.fn.system({ "kill", "-TERM", pid })
          status = vim.fn.jobwait({ browser_job }, 5000)[1]
        end
      end
    end
    if status == -1 then
      if allow_recorder_to_finish then
        vim.notify("contuts: browser recorder is finishing after Neovim exit", vim.log.levels.INFO)
      else
        vim.fn.jobstop(browser_job)
        vim.notify("contuts: browser recorder did not finish before timeout", vim.log.levels.WARN)
      end
    elseif vim.fn.filereadable(experiment_directory .. "/browser/DONE") ~= 1 then
      vim.wait(5000, function()
        return vim.fn.filereadable(experiment_directory .. "/browser/DONE") == 1
      end, 100)
    end
    if status ~= -1 and vim.fn.filereadable(experiment_directory .. "/browser/DONE") ~= 1 then
      vim.notify("contuts: browser recorder exited without completing artifacts", vim.log.levels.WARN)
    end
  end
  for name, job in pairs(M.target_jobs) do
    if job > 0 then
      if browser_job and M.experiment then
        vim.defer_fn(function()
          vim.fn.jobstop(job)
        end, 2000)
      else
        vim.fn.jobstop(job)
      end
    end
    M.target_jobs[name] = nil
  end
end

function M.stop_experiment()
  stop_target_jobs(false)
end

local function start_target_process(target)
  if not target.command then
    return
  end
  local directory = project_directory()
  local name = target_name(target)
  if M.target_jobs[name] then
    vim.fn.jobstop(M.target_jobs[name])
  end
  M.target_jobs[name] = vim.fn.jobstart({ "sh", "-lc", target.command }, {
    cwd = directory,
    stdout_buffered = false,
    stderr_buffered = false,
  })
  if M.target_jobs[name] <= 0 then
    M.target_jobs[name] = nil
    vim.notify("contuts: unable to start target " .. name, vim.log.levels.ERROR)
  end
end

local function has_target_kind(names, kind)
  for _, target in ipairs(selected_target_definitions(names)) do
    if (target.kind or target.type) == kind then
      return true
    end
  end
  return false
end

local function start_experiment_browser(names)
  local browser = (M.project.experiment or {}).browser or {}
  if not browser.command or not has_target_kind(names, "browser") then
    return
  end
  local command = string.format(
    "CONTUTS_EXPERIMENT_ID=%s CONTUTS_EXPERIMENT_DIR=%s %s",
    shell_quote(M.experiment.id),
    shell_quote(M.experiment.directory),
    expand_experiment_command(browser.command)
  )
  command = command .. " > " .. shell_quote(M.experiment.directory .. "/browser.log") .. " 2>&1"
  -- Put the recorder and Chromium in their own session. Otherwise VimLeavePre
  -- can send SIGHUP to Chromium before Playwright flushes its trace and HAR.
  local detached_command = "exec setsid --wait sh -lc " .. shell_quote(command)
  local delay = tonumber(browser.startDelayMs or 0) or 0
  vim.defer_fn(function()
    M.target_jobs["__contuts_experiment_browser"] = vim.fn.jobstart({ "sh", "-lc", detached_command }, {
      cwd = project_directory(),
      stdout_buffered = false,
      stderr_buffered = false,
      -- Keep Chromium alive long enough for the recorder to close its
      -- Playwright context and flush HAR/trace files during VimLeavePre.
      detach = true,
    })
    M.experiment.browser_started = M.target_jobs["__contuts_experiment_browser"] > 0
  end, delay)
end

local function target_configuration(target)
  local directory = project_directory()
  local configuration = vim.deepcopy(target.dap or {})
  local kind = target.kind or target.type
  if kind == "node" then
    configuration.type = configuration.type or "pwa-node"
    configuration.request = configuration.request or "launch"
    configuration.runtimeExecutable = configuration.runtimeExecutable or "pnpm"
    configuration.runtimeArgs = configuration.runtimeArgs or { "exec", "tsx", target.entry }
    configuration.console = configuration.console or "internalConsole"
  elseif kind == "browser" then
    configuration.type = configuration.type or "pwa-chrome"
    configuration.request = configuration.request or "launch"
    configuration.url = configuration.url or target.url
    configuration.webRoot = configuration.webRoot or directory
  end
  if not configuration.type then
    vim.notify("contuts: target " .. target_name(target) .. " has no kind or dap.type", vim.log.levels.ERROR)
    return nil
  end
  configuration.name = configuration.name or ("Contuts " .. target_name(target))
  configuration.cwd = configuration.cwd or directory
  configuration.sourceMaps = configuration.sourceMaps ~= false
  configuration.skipFiles = configuration.skipFiles or { "<node_internals>/**", "**/node_modules/**" }
  return configuration
end

local function stop_existing_sessions(callback)
  local dap = require("dap")
  if not next(dap.sessions()) then
    callback()
    return
  end
  dap.terminate({ all = true })
  vim.defer_fn(callback, 750)
end

launch_debug_targets = function(names)
  local dap = require("dap")
  local dapui = require("dapui")
  stop_existing_sessions(function()
    stop_target_jobs()
    dapui.open()
    for _, target in ipairs(selected_target_definitions(names)) do
      local target_delay = tonumber(target.startDelayMs or 0) or 0
      start_target_process(target)
      local experiment_browser = (M.project.experiment or {}).browser or {}
      local recorder_owns_browser = (target.kind or target.type) == "browser" and experiment_browser.command ~= nil
      local configuration = recorder_owns_browser and nil or target_configuration(target)
      if configuration then
        vim.defer_fn(function()
          dap.run(configuration)
        end, target_delay)
      end
    end
    start_experiment_browser(names)
  end)
end

function M.debug(targets)
  local dapui = require("dapui")
  M.active_target = true
  M.active_targets = targets or target_names()
  M.pending_debug_target = M.active_targets
  prepare_experiment(function()
    prepare_debug_breakpoints(function()
      dapui.open()
      write_experiment_metadata()
      vim.notify("Contuts experiment ready. Run :ContutsDebugStart to launch all configured targets.", vim.log.levels.INFO)
    end)
  end)
end

function M.start_debug()
  if not M.pending_debug_target then
    vim.notify("contuts: no pending debug plan", vim.log.levels.WARN)
    return
  end
  local targets = M.pending_debug_target
  M.pending_debug_target = nil
  save_breakpoints()
  write_experiment_metadata()
  launch_debug_targets(targets)
end

function M.debug_both()
  M.debug(target_names())
end

function M.reload_debug()
  if not M.active_target then
    return
  end
  prepare_debug_breakpoints(function()
    launch_debug_targets(M.active_targets)
  end)
end

local function select_debug_target(callback)
  local choices = { { name = "all", label = "All configured targets" } }
  for _, name in ipairs(target_names()) do
    table.insert(choices, { name = name, label = name })
  end
  vim.ui.select(choices, {
    prompt = "Contuts debug target",
    format_item = function(item)
      return item.label
    end,
  }, function(target)
    if target then
      callback(target.name == "all" and target_names() or { target.name })
    end
  end)
end

local function select_base(callback)
  local directory = project_directory()
  vim.system({ "git", "-C", directory, "log", "--first-parent", "--oneline", "--decorate", "--no-color", "HEAD" }, { text = true }, function(result)
    vim.schedule(function()
      if result.code ~= 0 then
        vim.notify("contuts: unable to list branch commits: " .. (result.stderr or "unknown error"), vim.log.levels.ERROR)
        return
      end
      local commits = {}
      for line in (result.stdout or ""):gmatch("[^\n]+") do
        local hash, label = line:match("^(%S+)%s+(.+)$")
        if hash then
          table.insert(commits, { hash = hash, label = line })
        end
      end
      if #commits == 0 then
        vim.notify("contuts: current branch has no commits to use as a base", vim.log.levels.ERROR)
        return
      end
      vim.ui.select(commits, {
        prompt = "Contuts base commit",
        format_item = function(commit)
          return commit.label
        end,
      }, function(commit)
        if commit then
          M.config.base = commit.hash
          callback(commit.hash)
        end
      end)
    end)
  end)
end

local function choose_base(callback)
  select_base(function()
    callback()
  end)
end

local function open_overview()
  if vim.api.nvim_buf_get_name(0) ~= "contuts://edits" then
    M.source_window = vim.api.nvim_get_current_win()
    vim.cmd("botright new")
    vim.api.nvim_buf_set_name(0, "contuts://edits")
  end
  refresh_overview()
end

local function refresh_session()
  open_overview()
end

local function set_comparison(arguments)
  if #arguments > 0 and arguments[1] ~= "" then
    M.config.base = arguments[1]
  end
  if #arguments > 1 and arguments[2] ~= "" then
    M.config.compare = arguments[2]
  end
end

function M.setup(options)
  M.config = vim.tbl_deep_extend("force", M.config, options or {})
  load_project_config()
  local comparison = M.project.comparison or {}
  M.config.base = comparison.base or M.project.base or M.config.base
  M.config.compare = comparison.current or comparison.compare or M.project.compare or M.config.compare
  vim.api.nvim_set_hl(0, "ContutsChanged", { link = "DiffChange" })
  vim.api.nvim_set_hl(0, "ContutsAdded", { link = "DiffAdd" })
  vim.api.nvim_set_hl(0, "ContutsRemoved", { link = "DiffDelete" })

  vim.api.nvim_create_user_command("Contuts", function(command)
    local arguments = vim.split(command.args, "%s+", { trimempty = true })
    if #arguments > 0 then
      set_comparison(arguments)
      refresh_session()
    else
      choose_base(refresh_session)
    end
  end, { nargs = "*" })

  vim.api.nvim_create_user_command("ContutsRefresh", function(command)
    set_comparison(vim.split(command.args, "%s+", { trimempty = true }))
    refresh_session()
  end, { nargs = "*" })

  vim.api.nvim_create_user_command("ContutsDebug", function()
    choose_base(function()
      select_debug_target(M.debug)
    end)
  end, {})

  vim.api.nvim_create_user_command("ContutsDebugTarget", function()
    select_debug_target(M.debug)
  end, {})

  vim.api.nvim_create_user_command("ContutsDebugStart", function()
    M.start_debug()
  end, {})

  vim.api.nvim_create_user_command("Contuts2Debug", function()
    choose_base(function()
      select_debug_target(M.debug)
    end)
  end, {})

  vim.api.nvim_create_user_command("ContutsBase", function()
    select_base(function()
      refresh_session()
    end)
  end, {})

  vim.api.nvim_create_user_command("ContutsDebugReload", function()
    M.reload_debug()
  end, {})

  vim.api.nvim_create_user_command("ContutsDebugRerun", function()
    select_experiment(M.rerun_experiment)
  end, {})

  vim.api.nvim_create_user_command("ContutsCoverage", function()
    M.open_coverage()
  end, {})

  local group = vim.api.nvim_create_augroup("ContutsDebugReload", { clear = true })
  vim.api.nvim_create_autocmd("BufWritePost", {
    group = group,
    pattern = { "*.ts", "*.tsx", "*.js", "*.jsx", "*.html", "*.css" },
    callback = function()
      M.reload_debug()
    end,
  })

  vim.api.nvim_create_autocmd("VimLeavePre", {
    group = group,
    callback = function()
      stop_target_jobs(true)
    end,
  })
end

return M
