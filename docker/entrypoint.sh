#!/bin/sh
# Run coverfill-all on COVERFILL_SCHEDULE with busybox crond. crond does not
# pass the container environment to its jobs, so save the settings to a file
# that the job reads.
set -eu
: "${COVERFILL_HOSTS:?set COVERFILL_HOSTS to the device addresses}"
: "${COVERFILL_SCHEDULE:=*/5 * * * *}"
export -p | grep -E '^export (COVERFILL_|TZ=)' > /run/coverfill.env || true
echo "$COVERFILL_SCHEDULE /usr/local/bin/coverfill-all > /proc/1/fd/1 2>&1" > /etc/crontabs/root
echo "coverfill $(coverfill -version | cut -d' ' -f2): checking $COVERFILL_HOSTS on '$COVERFILL_SCHEDULE'"
exec crond -f -l 8
