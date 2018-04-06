{{/* =% sh %= */}}

{{ $clearState := flag "clear-state" "bool" "Clear stored states" | prompt "Clear state?" "bool" true }}

{{ if $clearState }}
rm -rf $HOME/.infrakit/plugins/* # remove sockets, pid files, etc.
rm -rf $HOME/.infrakit/configs/global.config # for file based manager
# Since we are using file based leader detection, write the default name (manager1) to the leader file.
echo manager1 > $HOME/.infrakit/leader
{{ end }}

# The simulators are started up with different names to mimic different resources
INFRAKIT_MANAGER_CONTROLLERS=pool,resource \
infrakit plugin start manager:mystack vars group pool resource simulator:az1 \
	 --log 5 --log-stack --log-debug-V 500 \
	 --log-debug-match module=controller/pool \
	 --log-debug-match module=controller/internal \
	 --log-debug-match module=simulator/instance \
