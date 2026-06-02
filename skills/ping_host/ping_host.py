import sys
import json
import subprocess
import platform

def main():
    try:
        # Leer argumentos desde stdin
        input_data = sys.stdin.read()
        args = json.loads(input_data) if input_data.strip() else {}
        host = args.get("host")
        if not host:
            print(json.dumps({"error": "Host parameter is required"}))
            return

        # Determinar parámetro según OS
        param = '-n' if platform.system().lower() == 'windows' else '-c'
        # Hacemos 1 solo ping
        command = ['ping', param, '1', host]
        
        res = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        
        if res.returncode == 0:
            print(json.dumps({
                "status": "success",
                "message": f"Host {host} is reachable",
                "details": res.stdout
            }))
        else:
            print(json.dumps({
                "status": "failed",
                "message": f"Host {host} is unreachable",
                "error": res.stderr or res.stdout
            }))
    except Exception as e:
        print(json.dumps({"error": str(e)}))

if __name__ == "__main__":
    main()
