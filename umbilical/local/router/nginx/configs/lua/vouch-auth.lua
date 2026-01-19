function Set (list)
    local set = {}
    for _, l in ipairs(list) do set[l] = true end
    return set
end

function has_key(table, key)
    return table[key] ~= nil
end

function concat_keys(tab, sep)
    local r = ""
    for key, _ in pairs(tab) do
        r = r .. key .. sep
    end
    return r
end


function to_string(v)
    if type(v)=="table"   then
        return concat_keys(v, ":")
    end

    if type(v)=="string"   then
        return v
    end

    if v == nil   then
        return "nil"
    end

    return "unknown type"
end

local domain = "__{{infra.domains:f:u}}__"

local users = {
    ["__{{infra-emails:f:hh}}__"] = "*",
    ["__{{infra-emails:f:hz}}__"] = Set {"@", "stable-diffusion"},
    ["__{{infra-emails:f:cq}}__"] = Set {"@", "stable-diffusion"},
    ["__{{infra-emails:f:ck}}__"] = Set {"@", "stable-diffusion"},
}

ngx.log(ngx.ERR, "VOUCH USER: " .. ngx.var.auth_resp_x_vouch_user)

if ngx.var.auth_resp_x_vouch_user then
    local servers = users[ngx.var.auth_resp_x_vouch_user]
    ngx.log(ngx.ERR, "VOUCH User Servers: " .. to_string(servers))

    if servers == nil then
      ngx.exit(ngx.HTTP_FORBIDDEN)
      return
    end

    if servers == "*" then
        return
    end

    local server = ngx.var.server_name:gsub(domain, "")
    if server:sub(-1) == "." then
        server = server:sub(1, -2)
    end
    if server == "" then
        server = "@"
    end
    ngx.log(ngx.ERR, "VOUCH Current Server: " .. server)

    if not has_key(servers, server) then
        ngx.exit(ngx.HTTP_FORBIDDEN)
    end
else
    ngx.exit(ngx.HTTP_FORBIDDEN)
end
