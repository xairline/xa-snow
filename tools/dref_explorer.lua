--[[
    q&d explorer for snow and ice related datatrefs
]]

local drefs = {
    { dref = "sim/private/controls/wxr/snow_now", default = nil, max = 1.25 },
    { dref = "sim/private/controls/snow/luma_a", default = nil, max = 5 },
    { dref = "sim/private/controls/snow/luma_r", default = nil, max = 5 },
    { dref = "sim/private/controls/snow/luma_g", default = nil, max = 5 },
    { dref = "sim/private/controls/snow/luma_b", default = nil, max = 5 },
    { dref = "sim/private/controls/twxr/snow_area_scale", default = nil, max = 5 },
    { dref = "sim/private/controls/twxr/snow_area_width", default = nil, max = 5 },
    { dref = "sim/private/controls/twxr/ice/scale_albedo", default = nil, max = 5 },
    { dref = "sim/private/controls/twxr/ice/scale_decal", default = nil, max = 5 },
    { dref = "sim/private/controls/twxr/ice_area_scale", default = nil, max = 5 },
    { dref = "sim/private/controls/twxr/ice_area_width", default = nil, max = 5 },
    { dref = "sim/private/controls/wxr/ice_now", default = nil, max = 5 }
}

local alpha = 4
local snow_depth = 0

function sn_legacy(snow_depth)
    local v = 1.25
    if snow_depth > 0.001 then
        v = 1.05 - 1.127 * math.pow(snow_depth, 0.102)
        local cut_off = 0.08
        if v < cut_off then
            v = cut_off
        end
    end
end

function sn_new(snow_depth)
    if snow_depth >= 0.4 then
        return 0.04, 0.11
    end

    if snow_depth <= 0.01 then
        return 1.2, 0.25
    end

    local sd = { 0.01, 0.02, 0.03, 0.05, 0.10, 0.20, 0.40 }
    local sn = { 0.90, 0.70, 0.60, 0.30, 0.15, 0.06, 0.04 }
    local snaw = { 1.60, 1.41, 1.20, 0.52, 0.24, 0.14, 0.02 }
    for i, sd0 in pairs(sd) do
        sd1 = sd[i + 1]
        if sd0 <= snow_depth and snow_depth < sd1 then
            --logMsg(string.format("i: %d, sd0: %f", i, sd0))
            local x = (snow_depth - sd0) / (sd1 - sd0)
            v = sn[i] + x * (sn[i + 1] - sn[i])
            v2 = snaw[i] + x * (snaw[i + 1] - snaw[i])
            return v, v2
        end
    end

    logMsg(string.format("Should never happen: %f", snow_depth))
end

function win_build(wnd, x, y)
    if imgui.Button("Reset") then
        for i, dr in pairs(drefs) do
            set(dr.dref, dr.default)
        end
        snow_depth = 0.0
    end

    for i, dr in pairs(drefs) do
        local val = get(dr.dref)
        local changed, new_val = imgui.SliderFloat(dr.dref, val, 0.0, dr.max, "%0.3f")
        if changed then
            set(dr.dref, new_val)
        end
    end

    imgui.Separator()

    local changed_s, changed_a
    changed_s, snow_depth = imgui.SliderFloat("Snow depth (m)", snow_depth, 0.0, 2.0, "%0.3f")
    if changed_s then
        local v, v2 = sn_new(snow_depth)
        set("sim/private/controls/wxr/snow_now", v)
        set("sim/private/controls/twxr/snow_area_width", v2)
    end
end

function win_create()
    set("sim/private/controls/twxr/override", 1)

    win = float_wnd_create(900, 380, 1, true)

    float_wnd_set_imgui_builder(win, "win_build")
    float_wnd_set_onclose(win, "win_close")
end

function win_close(wnd)
    set("sim/private/controls/twxr/override", 0)
end

for i, dr in pairs(drefs) do
    dr.default = get(dr.dref)
    if dr.default > dr.max then
        dr.max = 1.5 * dr.default
    end
end

add_macro("dref_explorer", "win_create()")
win_create()
