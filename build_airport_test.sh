#!/bin/bash
g++ -o airport_test.exe -std=c++20 -DLOCAL_DEBUGSTRING -DTEST_AIRPORTS -DIBM -Wall -Werror \
	-Iservices/ -I SDK/CHeaders/XPLM services/collect_airports.cpp services/log_msg.cpp \
&& ./airport_test.exe

