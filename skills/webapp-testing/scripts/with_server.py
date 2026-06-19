#!/usr/bin/env python3
"""
Manages server lifecycle for testing.
Starts one or more servers, waits for them to be ready, runs a command, then cleans up.

Usage:
  python with_server.py --server "npm run dev" --port 5173 -- python test.py
  python with_server.py --server "go run ./cmd/server" --port 8080 --server "flutter run -d web-server --web-port 8081" --port 8081 -- python test.py
"""

import argparse
import socket
import subprocess
import sys
import time


def wait_for_port(port: int, timeout: int = 30) -> bool:
    start = time.time()
    while time.time() - start < timeout:
        try:
            with socket.create_connection(("localhost", port), timeout=1):
                return True
        except (ConnectionRefusedError, OSError):
            time.sleep(0.5)
    return False


def main():
    parser = argparse.ArgumentParser(description="Start servers, run a command, then clean up.")
    parser.add_argument("--server", action="append", default=[], help="Server command to run in background")
    parser.add_argument("--port", action="append", type=int, default=[], help="Port to wait for (matched by index to --server)")
    parser.add_argument("command", nargs=argparse.REMAINDER, help="Command to run after servers are ready")

    args = parser.parse_args()

    if len(args.server) != len(args.port):
        print("Error: --server and --port must be provided in matching pairs", file=sys.stderr)
        sys.exit(1)

    processes = []
    try:
        for cmd, port in zip(args.server, args.port):
            print(f"Starting server: {cmd} (waiting on port {port})")
            proc = subprocess.Popen(cmd, shell=True)
            processes.append(proc)
            if not wait_for_port(port):
                print(f"Error: server did not start on port {port} within 30s", file=sys.stderr)
                sys.exit(1)
            print(f"Server ready on port {port}")

        command = args.command
        if command and command[0] == "--":
            command = command[1:]

        if command:
            result = subprocess.run(command)
            sys.exit(result.returncode)

    finally:
        for proc in processes:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()


if __name__ == "__main__":
    main()
