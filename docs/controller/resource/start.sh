{{/* =% sh %= */}}

{{ $clearState := flag "clear-state" "bool" "Clear stored states" | prompt "Clear state?" "bool" true }}

{{ if $clearState }}
rm -rf $HOME/.infrakit/plugins/* # remove sockets, pid files, etc.
rm -rf $HOME/.infrakit/configs/global.config # for file based manager
# Since we are using file based leader detection, write the default name (manager1) to the leader file.
echo manager1 > $HOME/.infrakit/leader
{{ end }}

# The simulators are started up with different names to mimic different resources
INFRAKIT_MANAGER_CONTROLLERS=resource,pool \
infrakit plugin start manager:mystack vars group resource pool simulator:az1 simulator:az2 time \
	 --log 5 --log-stack --log-debug-V 1000 \
	 --log-debug-match method=DescribeInstances \
	 --log-debug-match module=simulator/instance \



