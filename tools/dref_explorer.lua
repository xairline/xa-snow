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
    changed_s, snow_depth = imgui.SliderFloat("Snow depth (m)", snow_depth, 0.0, 2.0, "%0.2f")
    if changed_s then
        local v = 1.25
        if snow_depth > 0.001 then
            v = 1.05 - 1.127 * math.pow(snow_depth, 0.102)
            local cut_off = 0.08
            if v < cut_off then v = cut_off end
        end
        set("sim/private/controls/wxr/snow_now", v)
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
