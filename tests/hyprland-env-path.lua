local source = "ryoku/hyprland/modules/env.lua"
local home = "/home/ryoku-test"
local local_bin = home .. "/.local/bin"

local function apply(path)
    local vars = { HOME = home, PATH = path }
    local environment = {
        hl = { env = function(name, value) vars[name] = value end },
        os = { getenv = function(name) return vars[name] end },
        io = { open = function() return nil end, popen = function() return nil end },
    }
    setmetatable(environment, { __index = _G })
    assert(loadfile(source, "t", environment))()
    return vars.PATH
end

local function check(initial, expected)
    local actual = apply(initial)
    assert(actual == expected,
        string.format("PATH %q: got %q, want %q", initial, actual, expected))
end

local expected = local_bin .. ":/usr/local/bin:/usr/bin"
check("/usr/local/bin:/usr/bin", expected)

local path = expected
for _ = 1, 100 do
    path = apply(path)
end
assert(path == expected, "PATH changed across repeated configuration loads")

check("/usr/bin:" .. local_bin .. ":/opt/bin:" .. local_bin,
    local_bin .. ":/usr/bin:/opt/bin")
check("/opt/.local/bin-tools:/usr/bin", local_bin .. ":/opt/.local/bin-tools:/usr/bin")
check("/usr/bin::/opt/bin:", local_bin .. ":/usr/bin::/opt/bin:")
check("", local_bin .. ":")
check(local_bin, local_bin)
check(local_bin .. ":" .. local_bin, local_bin)
check(local_bin .. ":", local_bin .. ":")
check(":" .. local_bin, local_bin .. ":")
check("/usr/bin:/usr/bin:" .. local_bin, local_bin .. ":/usr/bin:/usr/bin")

print("hyprland-env-path: OK")
