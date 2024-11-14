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
    local snowDepthTabLowerLimit = 0.01
    local snowDepthTabUpperLimit = 0.25
    local snowNowTabLowerLimit = 0.05
    local snowNowTabUpperLimit = 0.90
    local snowAreaWidthTabLowerLimit = 0.25
    local snowAreaWidthTabUpperLimit = 0.33

    if snow_depth >= snowDepthTabUpperLimit then
        return snowNowTabLowerLimit, snowAreaWidthTabUpperLimit -- snowAreaWidthTabLowerLimit
    end

    if snow_depth <= snowDepthTabLowerLimit then
        return 1.2, snowAreaWidthTabLowerLimit -- snowAreaWidthTabUpperLimit
    end

    local snowDepthTab = { snowDepthTabLowerLimit, 0.02, 0.03, 0.05, 0.10, 0.20, snowDepthTabUpperLimit }
    local snowNowTab = { snowNowTabUpperLimit, 0.70, 0.60, 0.30, 0.15, 0.06, snowNowTabLowerLimit }
    local snowAreaWidthTab = { snowAreaWidthTabUpperLimit, snowAreaWidthTabLowerLimit, snowAreaWidthTabLowerLimit, snowAreaWidthTabLowerLimit, snowAreaWidthTabLowerLimit, 0.29, snowAreaWidthTabLowerLimit }

    local snowNowValue = 1.2
    local snowAreaWidthValue = 0.25

    for i = 1, #snowDepthTab - 1 do
        local sd0 = snowDepthTab[i]
        local sd1 = snowDepthTab[i + 1]
        if sd0 <= snow_depth and snow_depth < sd1 then
            local x = (snow_depth - sd0) / (sd1 - sd0)
            snowNowValue = snowNowTab[i] + x * (snowNowTab[i + 1] - snowNowTab[i])

            -- Apply snow area width interpolation only when we have more than 20cm of snow
            if snow_depth >= 0.20 then
                snowAreaWidthValue = snowAreaWidthTab[i] + x * (snowAreaWidthTab[i + 1] - snowAreaWidthTab[i])
            end
            break
        end
    end

    return snowNowValue, snowAreaWidthValue
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
