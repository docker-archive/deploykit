{{/* =% sh %= */}}

{{ $clearState := flag "clear-state" "bool" "Clear stored states" | prompt "Clear state?" "bool" true }}

# Since we are using file based leader detection, write the default name (manager1) to the leader file.
echo manager1 > ~/.infrakit/leader

{{ if $clearState }}
rm -rf ~/.infrakit/plugins/* # remove sockets, pid files, etc.
rm -rf ~/.infrakit/configs/global.config # for file based manager
{{ end }}

# The simulators are started up with different names to mimic different resources
INFRAKIT_MANAGER_CONTROLLERS=resource \
infrakit plugin start manager:mystack vars group resource simulator:az1 simulator:az2 time \
	 --log 5 --log-stack --log-debug-V 400 \
	 --log-debug-match-exclude \
	 --log-debug-match module=core/fsm \
	 --log-debug-match module=simulator/instance \
	 --log-debug-match module=rpc/internal \
	 --log-debug-match module=run/manager \
	 --log-debug-match module=run/scope
