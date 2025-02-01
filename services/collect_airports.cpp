//
//    A contribution to https://github.com/xairline/xa-snow
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

#include <fstream>
#include "airport.h"
#include "xa-snow_c-impl.h"

std::vector<std::unique_ptr<Airport>> airports;

// SceneryPacks constructor
SceneryPacks::SceneryPacks(const std::string& xp_dir)
{
    std::string scpi_name(xp_dir + "/Custom Scenery/scenery_packs.ini");

    std::ifstream scpi(scpi_name);
    if (scpi.fail()) {
        log_msg("Can't open '%s'", scpi_name.c_str());
        return;
    }

    sc_paths.reserve(500);
    std::string line;

    while (std::getline(scpi, line)) {
        size_t i;
        if ((i = line.find('\r')) != std::string::npos)
            line.resize(i);

        if (line.find("SCENERY_PACK ") != 0 || line.find("*GLOBAL_AIRPORTS*") != std::string::npos)
            continue;

        // autoortho pretends every file exists but
        // reads give errors
        if (line.find("/z_ao_") != std::string::npos)
            continue;

        line.erase(0, 13);
        std::string sc_path;
        bool is_absolute = (line[0] == '/' || line.find(':') != std::string::npos);
        if (is_absolute)
            sc_path = line;
        else
            sc_path = xp_dir + "/" + line;

        // posixify
        for (unsigned i = 0; i < sc_path.size(); i++)
            if (sc_path[i] == '\\')
                sc_path[i] = '/';

        sc_paths.push_back(sc_path);
    }

    scpi.close();
    sc_paths.shrink_to_fit();
}

static void
SplitWords(std::string str, std::vector<std::string>& words)
{
	char *pch = strtok((char *)str.c_str(), " ");
	while (pch != NULL) {
		words.push_back(pch);
		pch = strtok (NULL, " ");
	}
}

// go through apt.dat and collect runways from 100 lines
static bool
ParseAptDat(const std::string& fn, Airport& arpt)
{
    std::ifstream apt(fn);
    if (apt.fail())
        return false;

    log_msg("Processing '%s'", fn.c_str());

    std::string line;
    line.reserve(2000);          // can be quite long

    while (std::getline(apt, line)) {
        size_t i = line.find('\r');
        if (i != std::string::npos)
            line.resize(i);

		//1    681 0 0 ENGM Oslo Gardermoen
        if (line.find("1 ") == 0) {
			//log_msg("%s", line.c_str());
			int ofs;
			sscanf(line.c_str(), "%*d %*d %*d %*d %n", &ofs);
			if (ofs < (int)line.size())
				line.erase(0, ofs);
			arpt.name = line;
			continue;
		}

		//100 45.11 15 0 0.00 1 3 0 01L  60.18499584  011.07373840 0 148 3 1 0 0 19R  60.21615335  011.09170422 0 140 3 2 1 0
        if (line.find("100 ") == 0) {
			std::vector<std::string> words;
			SplitWords(line, words);

			int code = std::atoi(words[2].c_str());
			code %= 100;
			if (code != 15)
				continue;

            //log_msg("%s", line.c_str());
			Runway rwy;
			rwy.name = words[8];
			rwy.end1.lat = std::atof(words[9].c_str());
			rwy.end1.lon = std::atof(words[10].c_str());
			rwy.end2.lat = std::atof(words[18].c_str());
			rwy.end2.lon = std::atof(words[19].c_str());
			arpt.runways.push_back(rwy);
		}
    }

    apt.close();
    return true;
}

bool
CollectAirports(const std::string& xp_dir)
{
	SceneryPacks scp(xp_dir);
	if (scp.sc_paths.size() == 0) {
		log_msg("Can't collect scenery_packs.ini");
		return false;
	}

	airports.reserve(50);

	for (auto & path : scp.sc_paths) {
		airports.emplace_back(std::make_unique<Airport>());
		Airport &arpt = *airports.back();
		ParseAptDat(path + "Earth nav data/apt.dat", arpt);
		if (arpt.runways.size() == 0)
			airports.pop_back();
		else
			arpt.runways.shrink_to_fit();
	}

	for (auto & arpt : airports) {
		log_msg("%s", arpt->name.c_str());
		for (auto & rw : arpt->runways) {
			log_msg("  rw: %-3s, end1: (%0.4f, %0.4f), end2: (%0.4f, %0.4f)",
				    rw.name.c_str(), rw.end1.lat, rw.end1.lon, rw.end2.lat, rw.end2.lon);
		}
	}

	airports.shrink_to_fit();
	return true;
}

#ifdef TEST_AIRPORTS
int main()
{
	bool res = CollectAirports("e:/X-Plane-12-test");
	log_msg("res: %d", res);
	return 0;
}
#endif
