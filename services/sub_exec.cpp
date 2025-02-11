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

#include <string>
#include <cstdlib>
#ifdef _WIN32
#include <windows.h>
#include <system_error>
#endif

#include "xa-snow.h"

int
sub_exec(const std::string& command)
{
    std::string output;

#ifdef _WIN32
    std::error_code ec;
    STARTUPINFO si;
    ZeroMemory(&si, sizeof(si));
    si.cb = sizeof(si);
    si.dwFlags |= STARTF_USESTDHANDLES;
    si.dwFlags |= STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    PROCESS_INFORMATION pi;
    ZeroMemory(&pi, sizeof(pi));

    SECURITY_ATTRIBUTES security_attributes;
    ZeroMemory(&security_attributes, sizeof(security_attributes));
    security_attributes.nLength = sizeof(security_attributes);
    security_attributes.bInheritHandle = TRUE;

    // Create pipes for the child's STDOUT and STDIN.
    HANDLE hStdOutRead, hStdOutWrite;
    if (!CreatePipe(&hStdOutRead, &hStdOutWrite, &security_attributes, 0))
    {
        ec = std::error_code(GetLastError(), std::system_category());
        return -1;
    }

    // Ensure the read handle to the pipe for STDOUT is not inherited.
    if (!SetHandleInformation(hStdOutRead, HANDLE_FLAG_INHERIT, 0))
    {
        ec = std::error_code(GetLastError(), std::system_category());
        return -1;
    }

    si.hStdOutput = hStdOutWrite;
    si.hStdError = hStdOutWrite;

    // Start the child process.
    if (!CreateProcess(NULL,
        const_cast<char*>(command.c_str()),
        NULL,
        NULL,
        TRUE,
        0,
        NULL,
        NULL,
        &si,
        &pi)){
        //logMessage(simple_format("CreateProcess failed %.", GetLastError()));
        log_msg("CreateProcess failed");
        ec = std::error_code(GetLastError(), std::system_category());
        CloseHandle(hStdOutWrite);
        CloseHandle(hStdOutRead);
        return -1;
    }

    // Close handles to the child's STDOUT and stdin.
    CloseHandle(hStdOutWrite);

    // Read from pipe and invoke callback.
    char buffer[256];
    DWORD readBytes;
    while (ReadFile(hStdOutRead, buffer, sizeof(buffer) - 1, &readBytes, NULL) && readBytes > 0) {
        buffer[readBytes] = '\0';
        output += buffer;
    }

    // Close remaining handles.
    CloseHandle(hStdOutRead);
    WaitForSingleObject(pi.hProcess, INFINITE);
    DWORD exit_code;
    if (!GetExitCodeProcess(pi.hProcess, (LPDWORD)&exit_code)) {
        ec = std::error_code(GetLastError(), std::system_category());
        CloseHandle(pi.hThread);
        CloseHandle(pi.hProcess);
        return -1;
    }

    CloseHandle(pi.hThread);
    CloseHandle(pi.hProcess);

    ec.clear();
    if (exit_code != 0)
        log_msg("sub_exec output: '%s', exit_code: %ld", output.c_str(), exit_code);

    return exit_code;

#else
    int exit_code = 0;
    // For Unix-like systems
    std::array<char, 128> buffer;

    FILE* pipe = popen(command.c_str(), "r");
    if (!pipe) throw std::runtime_error("popen() failed!");

    while (fgets(buffer.data(), buffer.size(), pipe) != nullptr) {
        result += buffer.data();
    }
    exitCode = pclose(pipe);

    if (exitCode != 0) {
        // Assuming Logger is defined and has an error method
        // g.Logger.Errorf("Error getting snow depth: %d, %s", exitCode, result);
        std::cerr << "Error getting snow depth: " << exitCode << ", " << result << std::endl;
        throw std::runtime_error("Command execution failed");
    }
    std::cout << output << "\n";
    return 1;
#endif
}

#if 0
#include <iostream>
int
main()
{
    int res = exec("bin\\WIN32wgrib2.exe");
    std::cout << res << std::endl;
}
#endif