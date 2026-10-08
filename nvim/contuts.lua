local M = {}
local open_edit

M.config = {
  command = "contuts",
  directory = nil,
  base = "HEAD~1",
  compare = "working-tree",
}

local function display_name(edit)
  local value = edit.value and (" " .. edit.value) or ""
  local location = edit.startLine and (" line " .. edit.startLine) or ""
  return string.format("%s %s%s%s", edit.kind, edit.nodeKind, value, location)
end

local function refresh()
  local directory = M.config.directory or vim.fn.getcwd()
  local command = { M.config.command, "--nvim-json", directory, M.config.base, M.config.compare }
  vim.system(command, { text = true }, function(result)
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

function M.render(data)
  local buffer = vim.api.nvim_get_current_buf()
  local lines = {
    string.format("CONTUTS  %s -> %s", data.base, data.compare),
    "Press <CR> on an edit to open its file. Press r to refresh. q to close.",
    "",
  }
  local locations = {}
  for _, file in ipairs(data.files or {}) do
    table.insert(lines, file.path)
    local visible = {}
    for _, edit in ipairs(file.edits or {}) do
      visible[edit.nodeId] = edit
    end
    for _, edit in ipairs(file.edits or {}) do
      if not edit.hidden then
        local depth = 0
        for _, ancestor in ipairs(edit.ancestorIds or {}) do
          if visible[ancestor] and not visible[ancestor].hidden then
            depth = depth + 1
          end
        end
        table.insert(lines, string.rep("  ", depth + 1) .. display_name(edit))
        locations[#lines] = { path = file.path, line = edit.startLine or 1 }
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
  vim.keymap.set("n", "<CR>", open_edit, { buffer = buffer, silent = true })
  vim.keymap.set("n", "r", refresh, { buffer = buffer, silent = true })
  vim.keymap.set("n", "q", "<cmd>close<cr>", { buffer = buffer, silent = true })
end

open_edit = function()
  local location = (vim.b.contuts_locations or {})[vim.fn.line(".")]
  if not location then
    return
  end
  local directory = M.config.directory or vim.fn.getcwd()
  vim.cmd("edit " .. vim.fn.fnameescape(directory .. "/" .. location.path))
  vim.api.nvim_win_set_cursor(0, { math.max(1, location.line), 0 })
end

function M.setup(options)
  M.config = vim.tbl_deep_extend("force", M.config, options or {})
  vim.api.nvim_create_user_command("Contuts", function()
    vim.cmd("botright new")
    vim.api.nvim_buf_set_name(0, "contuts://edits")
    refresh()
  end, {})
  vim.api.nvim_create_user_command("ContutsRefresh", refresh, {})
  vim.keymap.set("n", "<CR>", open_edit, { buffer = true, silent = true })
  vim.keymap.set("n", "r", refresh, { buffer = true, silent = true })
  vim.keymap.set("n", "q", "<cmd>close<cr>", { buffer = true, silent = true })
end

return M
