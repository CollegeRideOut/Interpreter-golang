local M = {}

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
M.frontend_job = nil
M.breakpoint_buffers = {}
M.active_target = nil
M.last_plan = nil
M.project = {
  frontend = {
    command = "pnpm dev:web",
    url = "http://localhost:5173",
  },
  backend = {},
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

local function clear_breakpoints()
  local breakpoints = require("dap.breakpoints")
  breakpoints.clear()
  M.breakpoint_buffers = {}
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
            seen[line] = true
          end
        end
        M.breakpoint_buffers[buffer] = true
      end
    end
  end
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
      callback()
    end)
  end)
end

local function stop_frontend()
  if M.frontend_job and M.frontend_job > 0 then
    vim.fn.jobstop(M.frontend_job)
    M.frontend_job = nil
  end
end

local function start_frontend()
  stop_frontend()
  local directory = project_directory()
  M.frontend_job = vim.fn.jobstart({ "sh", "-lc", M.project.frontend.command }, {
    cwd = directory,
    stdout_buffered = false,
    stderr_buffered = false,
  })
  if M.frontend_job <= 0 then
    vim.notify("contuts: unable to start frontend command", vim.log.levels.ERROR)
  end
end

local function backend_configuration()
  local directory = project_directory()
  if not M.project.backend.entry then
    vim.notify("contuts: .contuts.json has no backend.entry", vim.log.levels.ERROR)
    return nil
  end
  return {
    type = "pwa-node",
    request = "launch",
    name = "Contuts TypeScript API",
    cwd = directory,
    runtimeExecutable = "pnpm",
    runtimeArgs = { "exec", "tsx", M.project.backend.entry },
    sourceMaps = true,
    skipFiles = { "<node_internals>/**", "**/node_modules/**" },
    console = "internalConsole",
  }
end

local function browser_configuration()
  return {
    type = "pwa-chrome",
    request = "launch",
    name = "Contuts Narrativo Web",
    url = M.project.frontend.url,
    webRoot = project_directory(),
    sourceMaps = true,
    skipFiles = { "<node_internals>/**", "**/node_modules/**" },
  }
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

local function launch_debug_target(target)
  local dap = require("dap")
  local dapui = require("dapui")
  stop_existing_sessions(function()
    dapui.open()
    if target == "backend" or target == "both" then
      local configuration = backend_configuration()
      if configuration then
        dap.run(configuration)
      end
    end
    if target == "frontend" or target == "both" then
      start_frontend()
      vim.defer_fn(function()
        dap.run(browser_configuration())
      end, 1500)
    end
  end)
end

function M.debug(target)
  local dapui = require("dapui")
  M.active_target = target
  M.pending_debug_target = target
  prepare_debug_breakpoints(function()
    dapui.open()
    vim.notify("Contuts breakpoints ready. Review them, then run :ContutsDebugStart", vim.log.levels.INFO)
  end)
end

function M.start_debug()
  if not M.pending_debug_target then
    vim.notify("contuts: no pending debug plan", vim.log.levels.WARN)
    return
  end
  local target = M.pending_debug_target
  M.pending_debug_target = nil
  launch_debug_target(target)
end

function M.debug_both()
  M.debug("both")
end

function M.reload_debug()
  if not M.active_target then
    return
  end
  local dap = require("dap")
  prepare_debug_breakpoints(function()
    if M.active_target == "both" then
      dap.terminate({ all = true })
      start_frontend()
      vim.defer_fn(function()
        local backend = backend_configuration()
        if backend then
          dap.run(backend)
        end
        vim.defer_fn(function()
          dap.run(browser_configuration())
        end, 1500)
      end, 500)
    elseif dap.session() then
      dap.terminate()
      vim.defer_fn(function()
        dap.run_last()
      end, 500)
    end
  end)
end

local function select_debug_target(callback)
  vim.ui.select({ "frontend", "backend", "both" }, { prompt = "Contuts debug target" }, function(target)
    if target then
      callback(target)
    end
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
  M.config.base = "HEAD~1"
  M.config.compare = "working-tree"
  open_overview()
end

function M.setup(options)
  M.config = vim.tbl_deep_extend("force", M.config, options or {})
  load_project_config()
  vim.api.nvim_set_hl(0, "ContutsChanged", { link = "DiffChange" })
  vim.api.nvim_set_hl(0, "ContutsAdded", { link = "DiffAdd" })
  vim.api.nvim_set_hl(0, "ContutsRemoved", { link = "DiffDelete" })

  vim.api.nvim_create_user_command("Contuts", function()
    refresh_session()
  end, {})

  vim.api.nvim_create_user_command("ContutsRefresh", function()
    refresh_session()
  end, {})

  vim.api.nvim_create_user_command("ContutsDebug", function()
    select_debug_target(M.debug)
  end, {})

  vim.api.nvim_create_user_command("ContutsDebugStart", function()
    M.start_debug()
  end, {})

  vim.api.nvim_create_user_command("Contuts2Debug", function()
    refresh_session()
    select_debug_target(M.debug)
  end, {})

  vim.api.nvim_create_user_command("ContutsDebugReload", function()
    M.reload_debug()
  end, {})

  local group = vim.api.nvim_create_augroup("ContutsDebugReload", { clear = true })
  vim.api.nvim_create_autocmd("BufWritePost", {
    group = group,
    pattern = { "*.ts", "*.tsx", "*.js", "*.jsx", "*.html", "*.css" },
    callback = function()
      M.reload_debug()
    end,
  })
end

return M
