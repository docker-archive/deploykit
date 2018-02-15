{{/* =% sh %= */}}

# The simulators are started up with different names to mimic different resources
infrakit plugin start manager vars group gc simulator:docker simulator:vm --log 5 --log-debug-V 500
