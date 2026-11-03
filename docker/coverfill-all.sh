#!/bin/sh
# Run coverfill once for each address in COVERFILL_HOSTS. A device that is
# asleep or not in File Transfer fails with "device not reachable" until the
# next run.
[ -f /run/coverfill.env ] && . /run/coverfill.env
exec 9> /run/coverfill.lock
flock -n 9 || { echo "coverfill: the previous run is still busy, skipped"; exit 0; }
for host in $(echo "$COVERFILL_HOSTS" | tr ',' ' '); do
	echo "== $(date '+%F %T') $host"
	coverfill -host "$host" ${COVERFILL_ARGS:-}
done
