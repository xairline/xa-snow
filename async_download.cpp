//
//    A contribution to https://github.com/xairline/xa-snow by zodiac1214
//
//    Copyright (C) 2025  Holger Teutsch
//
//    This library is free software; you can redistribute it and/or
//    modify it under the terms of the GNU Lesser General Public
//    License as published by the Free Software Foundation; either
//    version 2.1 of the License, or (at your option) any later version.
//
//    This library is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
//    Lesser General Public License for more details.
//
//    You should have received a copy of the GNU Lesser General Public
//    License along with this library; if not, write to the Free Software
//    Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301
//    USA
//

#include <cstdio>
#include <ctime>
#include <array>
#include <string>
#include <mutex>
#include <future>
#include <chrono>
#include <filesystem>
#include <curl/curl.h>

#include "xa-snow.h"


// in:  user specified time
// out: url, cycle time, cycle num
static std::tuple<std::string, std::tm, int>
GetDownloadUrl(bool sys_time, const std::tm utime_utc)
{
    // Adjusted time considering publish delay
    std::tm ctime_utc = utime_utc;
    ctime_utc.tm_hour -= 4;
    ctime_utc.tm_min -= 25;
    std::mktime(&ctime_utc); // Normalize the time structure

    char buffer[1000];
    std::strftime(buffer, sizeof(buffer), "%Y-%m-%d %H:%M:%S", &ctime_utc);
    log_msg("adjusted utime_utc: %s", buffer);

    const static std::array<int, 4> cycles = {0, 6, 12, 18};
    int cycle = 0;
    for (int c : cycles) {
        if (ctime_utc.tm_hour >= c) {
            cycle = c;
        }
    }

    int adjs = 0;
    if (ctime_utc.tm_mday != utime_utc.tm_mday) {
        adjs = 24;
    }
    int forecast = (adjs + utime_utc.tm_hour - cycle) / 3 * 3;

    snprintf(buffer, sizeof(buffer), "%d%02d%02d", ctime_utc.tm_year + 1900, ctime_utc.tm_mon + 1, ctime_utc.tm_mday);
    std::string cycleDate(buffer);

    if (sys_time) {
        snprintf(buffer, sizeof(buffer), "gfs.t%02dz.pgrb2.0p25.f0%02d", cycle, forecast);
        std::string filename(buffer);
        log_msg("NOAA Filename: '%s', %d, %d", filename.c_str(), cycle, forecast);

        snprintf(buffer, sizeof(buffer), "https://nomads.ncep.noaa.gov/cgi-bin/filter_gfs_0p25.pl?dir=%%2Fgfs.%s%%2F%02d%%2Fatmos&file=%s&var_SNOD=on&all_lev=on", cycleDate.c_str(), cycle, filename.c_str());
        return {buffer, ctime_utc, cycle};
    } else {
        forecast = 6; // TODO: for now
        snprintf(buffer, sizeof(buffer), "gfs.0p25.%s%02d.f0%02d.grib2", cycleDate.c_str(), cycle, forecast);
        std::string filename(buffer);
        log_msg("GITHUB Filename: '%s', %d, %d", filename.c_str(), cycle, forecast);

        snprintf(buffer, sizeof(buffer), "https://github.com/xairline/weather-data/releases/download/daily/%s", filename.c_str());
        return {buffer, ctime_utc, cycle};
    }
}

static bool
http_get(const char *url, FILE *f, int timeout)
{
    CURL *curl;
    CURLcode res;
    curl_global_init(CURL_GLOBAL_ALL);
    curl = curl_easy_init();
    if(curl == NULL)
        return 0;

    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, timeout);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, fwrite);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, f);

    curl_easy_setopt(curl, CURLOPT_HTTPGET, 1L);
    curl_easy_setopt(curl, CURLOPT_FOLLOWLOCATION, 1L);
    res = curl_easy_perform(curl);

    // Check for errors
    if(res != CURLE_OK) {
        log_msg("curl_easy_perform() failed: %s\n", curl_easy_strerror(res));
        curl_easy_cleanup(curl);
        curl_global_cleanup();
        return false;
    }

    curl_off_t dl_size;
    res = curl_easy_getinfo(curl, CURLINFO_SIZE_DOWNLOAD_T , &dl_size);
    if(res == CURLE_OK)
        log_msg("Downloaded %d bytes", (int)dl_size);

    curl_easy_cleanup(curl);
    curl_global_cleanup();
    return true;
}

std::string gribFileFolder;

std::string
DownloadGribFile(bool sys_time, int day, int month, int hour)
{
    log_msg("downloadGribFile: Using system time: %d, month: %d, day: %d, hour: %d", sys_time, day, month, hour);

    std::time_t now = std::time(nullptr);
    std::tm now_tm = *std::localtime(&now);
    std::time_t provided_time = now;


    if (! sys_time) {
        std::tm provided_tm = now_tm;
        provided_tm.tm_mon = month - 1; // Adjust month
        provided_tm.tm_mday = day;
        provided_tm.tm_hour = hour;
        provided_tm.tm_min = 0;
        provided_tm.tm_sec = 0;

        provided_time = std::mktime(&provided_tm);
        std::time_t twenty_four_hours_ago = now - 24 * 60 * 60;

        // Check if the provided time is not within the last 24 hours
        if (provided_time <= twenty_four_hours_ago || provided_time > now) {
            // historic mode
            int year = now_tm.tm_year;
            int current_month = now_tm.tm_mon + 1;
            int current_day = now_tm.tm_mday;
            int current_hour = now_tm.tm_hour;

            if ((month > current_month) ||
                (month == current_month && day > current_day) ||
                (month == current_month && day == current_day && hour > current_hour)) {
                // future month/day/hour -> use previous year
                year--;
            }
            provided_tm.tm_year = year;
            provided_time = std::mktime(&provided_tm);
        } else {
            log_msg("The provided time is within the last 24 hours. Using system time.");
            sys_time = true;
            provided_time = now;
        }
    }

    // now convert provided time to UTC
    auto ptime_utc_tm = *std::gmtime(&provided_time);
    char buffer[500];
    strftime(buffer, sizeof(buffer), "%Y-%m-%d-%H:%M:%S", &ptime_utc_tm);
    log_msg("provided time (UTC): %s", buffer);

    auto [url, ctime_utc_tm, cycle] = GetDownloadUrl(sys_time, ptime_utc_tm);
    log_msg("Downloading GRIB file from '%s'", url.c_str());

    // Get grib file's date date in yyyy-mm-dd format
    strftime(buffer, sizeof(buffer), "%Y-%m-%d", &ctime_utc_tm);
    char fn_buffer[500];
    snprintf(fn_buffer, sizeof(fn_buffer), "%s/%s_%d_noaa.grib2", gribFileFolder.c_str(), buffer, cycle);
    std::string grib_file_path = fn_buffer;
    log_msg("GRIB file path: '%s'", grib_file_path.c_str());

    // if file does not exist, download
    if (!std::filesystem::exists(grib_file_path)) {
        FILE *out = fopen(grib_file_path.c_str(), "wb");
        if (out == NULL) {
            log_msg("ERROR: can't create '%s'", grib_file_path.c_str());
            return "";
        }
        if (http_get(url.c_str(), out, 10))
            log_msg("GRIB File downloaded successfully");
        else {
            log_msg("GRIB File download failed");
            grib_file_path = "";
        }

        fclose(out);
    }

    return grib_file_path;
}

// everything is synchronously fired by the flightloop so we don't need mutexes
static bool download_active;
std::future<bool> download_future;

static bool
DownloadAndProcessGribFile(bool sys_time, int day, int month, int hour)
{
    std::string grib_file_path = DownloadGribFile(sys_time, day, month, hour);
    if (grib_file_path.size() == 0)
        return false;

#if IBM == 1
    std::string bin_path = "bin/WIN32wgrib2.exe";
#else
/*
	}
	if myOs == "linux" {
		executablePath = filepath.Join(g.binPath, "linux-wgrib2")
	}
	if myOs == "darwin" {
		executablePath = filepath.Join(g.binPath, "OSX11wgrib2")
	}

 */
#endif
    std::string snow_csv_name = "snod.csv";

	// export grib file to csv
	// 0:3600:0.1 means scan longitude from 0, 3600 steps with step 0.1 degree
	// -90:1800:0.1 means scan latitude from -90, 1800 steps with step 0.1 degree
	std::string cmd = bin_path + " -s -lola 0:3600:0.1 -90:1800:0.1 " + snow_csv_name + " spread " + grib_file_path + " -match_fs SNOD";
    log_msg("cmd:'%s'", cmd.c_str());
    return (0 == sub_exec(cmd));
}

static bool
DownloadAndProcess(bool sys_time, int day, int month, int hour)
{
    for (int i = 0; i < 3; i++) {
        bool res = DownloadAndProcessGribFile(sys_time, month, day, hour);
        if (!res) {
            log_msg("Download grib file failed, retry: %d", i);
        } else {
            log_msg("Download and process grib file successfully");
            return true;
        }
    }
    log_msg("grib download/process: all retry failed");
    return false;
}

// start download in the background
void
StartAsyncDownload(bool sys_time, int day, int month, int hour)
{
    if (download_active) {
        log_msg("Download is already in progress, request ignored");
    }

    download_future = std::async(std::launch::async, DownloadAndProcess, sys_time, day, month, hour);
    download_active = true;
}

// return true on the transition from download_active to not active
bool
CheckAsyncDownload()
{
    if (!download_active)
        return false;

    if (std::future_status::ready != download_future.wait_for(std::chrono::seconds::zero()))
        return false;

    download_active = false;
    bool res = download_future.get();
    log_msg("Download status: %d", res);
    return true;
}

#include <iostream>
std::string xp_dir;
static void
flightloop_emul()
{
    while (! CheckAsyncDownload()) {
        log_msg("... waiting for async download");
        std::this_thread::sleep_for(std::chrono::seconds(3));
    }
}

//  g++ -std=c++20 -Wall -Iservices -ISDK/CHeaders/XPLM -DIBM=1 -DLOCAL_DEBUGSTRING async_download.cpp services/log_msg.cpp services/sub_exec.cpp -lcurl
int main()
{
    xp_dir = ".";
    gribFileFolder = xp_dir;

    StartAsyncDownload(true, 0, 0, 0);
    flightloop_emul();
    std::cout << "-------------------------------------------------\n\n";

#if 0
    StartAsyncDownload(false, 10, 2, 21);
    flightloop_emul();
    std::cout << "-------------------------------------------------\n\n";

    StartAsyncDownload(false, 20, 2, 22);
    flightloop_emul();
    std::cout << "-------------------------------------------------\n\n";

    StartAsyncDownload(false, 20, 1, 10);
    flightloop_emul();
#endif

    return 0;
}
