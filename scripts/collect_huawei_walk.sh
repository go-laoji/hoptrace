#!/usr/bin/env bash
# 采集华为交换机/防火墙的 SNMP walk 样本(数字 OID),用于离线回放测试与 OID 校准(Issue #13)
# 用法:
#   SNMP v2c: ./scripts/collect_huawei_walk.sh -h 10.0.0.1 -v 2c -c public -o testdata/s5720.walk
#   SNMP v3 : ./scripts/collect_huawei_walk.sh -h 10.0.0.1 -v 3 -u rouser -a SHA -A authpass -x AES -X privpass -o testdata/usg6510e.walk
set -euo pipefail

HOST=""
VER="2c"
COMM="public"
USER=""
AUTH_PROTO="SHA"
AUTH_PASS=""
PRIV_PROTO="AES"
PRIV_PASS=""
OUT="device.walk"

while getopts "h:v:c:u:a:A:x:X:o:" opt; do
  case "$opt" in
    h) HOST="$OPTARG" ;;
    v) VER="$OPTARG" ;;
    c) COMM="$OPTARG" ;;
    u) USER="$OPTARG" ;;
    a) AUTH_PROTO="$OPTARG" ;;
    A) AUTH_PASS="$OPTARG" ;;
    x) PRIV_PROTO="$OPTARG" ;;
    X) PRIV_PASS="$OPTARG" ;;
    o) OUT="$OPTARG" ;;
    *) echo "Unknown option"; exit 1 ;;
  esac
done

if [[ -z "$HOST" ]]; then
  echo "Usage: $0 -h <ip> [-v 2c|3] [-c community] [-u user -a SHA -A auth -x AES -X priv] [-o out.walk]"
  exit 1
fi

ARGS=(-On -t 3 -r 1)
if [[ "$VER" == "3" ]]; then
  ARGS+=(-v3 -l authPriv -u "$USER" -a "$AUTH_PROTO" -A "$AUTH_PASS" -x "$PRIV_PROTO" -X "$PRIV_PASS")
else
  ARGS+=(-v2c -c "$COMM")
fi

OIDS=(
  "1.3.6.1.2.1.1"                    # SNMPv2-MIB system
  "1.3.6.1.2.1.2.2"                  # IF-MIB ifTable
  "1.3.6.1.2.1.31.1.1"               # IF-MIB ifXTable
  "1.3.6.1.2.1.4.20"                 # IP-MIB ipAddrTable
  "1.3.6.1.2.1.4.22"                 # RFC1213 ARP
  "1.3.6.1.2.1.4.35"                 # IP-MIB ARP
  "1.2.840.10006.300.43.1.2.1.1.13"  # IEEE8023-LAG
  "1.3.6.1.2.1.17.1"                 # BRIDGE-MIB base
  "1.3.6.1.2.1.17.4.3"               # BRIDGE-MIB FDB
  "1.3.6.1.2.1.17.7.1.2.2"           # Q-BRIDGE-MIB FDB
  "1.0.8802.1.1.2.1.3"               # LLDP local
  "1.0.8802.1.1.2.1.4"               # LLDP remote
  "1.3.6.1.2.1.47.1.1.1.1.5"         # ENTITY class
  "1.3.6.1.2.1.47.1.1.1.1.11"        # ENTITY serial
  "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.5"  # Huawei CPU
  "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.7"  # Huawei Mem
  "1.3.6.1.4.1.2011.5.25.31.1.1.1.1.11" # Huawei Temp
)

mkdir -p "$(dirname "$OUT")"
: > "$OUT"
for oid in "${OIDS[@]}"; do
  echo "# walking $oid ..." >&2
  snmpwalk "${ARGS[@]}" "$HOST" "$oid" >> "$OUT" || true
done
echo "Saved walk sample to $OUT ($(wc -l < "$OUT") lines)"
