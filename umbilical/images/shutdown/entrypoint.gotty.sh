#!/usr/bin/env bash
set -e

session="main"
node=$(hostname)
auto="$1"

if [ "${auto}" == "true" ]; then
	echo "Auto shutdown. node: ${node}, wait 30 minutes for interruption if any ..."
	sleep 1800

	now=$(date +"%H:%M")
	start_time="00:00"
	end_time="06:00"

	if [[ "$now" < "$start_time" ]] || [[ "$now" > "$end_time" ]]; then
		echo "The time ${now} is not between ${start_time} and ${end_time}. Shutdown aborted."
		exit 1
	fi
fi

if ! tmux has-session -t "${session}" 2>/dev/null; then
	echo "Session does not exist."

	if [ ! "${auto}" == "true" ]; then
		echo "Really want to shutdown? (y/N)"
		read -r -n 1 -s answer
		if [ "${answer}" != "Y" ] && [ "${answer}" != "y" ]; then
			echo "Shutdown aborted."
			exit 0
		fi
	fi

	echo "Shutting down. node: ${node} ..."
	tmux new -d -A -s "${session}" bash -c "systemctl poweroff"
fi

tmux attach-session -t "${session}" -r
