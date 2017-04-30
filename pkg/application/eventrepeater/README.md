# Infrakit Event Repeater Application
This is a sample of Infrakit Application.
It enable to repeat events from event plugin to MQTT brocker.

## Get Start

### Prepare
Start event plugin and mqtt broker
```
$ ./build/infrakit-event-time
$ docker run -it --rm -p 1883:1883 eclipse-mosquitto
```

## Run event repeater

```


___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


Event Repeater service

Usage:
  infrakit util event-repeater [flags]

Flags:
      --allowall              Allow all event from source and repeat the event to sink as same topic name. default: false
      --listen string         Application listen host:port
      --log int               Logging level. 0 is least verbose. Max is 5 (default 4)
      --name string           Application name to advertise for discovery (default "app-event-repeater")
      --sink string           Event sink address. default: localhost:1883 (default "localhost:1883")
      --sinkprotocol string   Event sink protocol. Now only mqtt and stderr is implemented. (default "mqtt")
      --source string         Event sourve address. (default "event-plugin")

Global Flags:
  -H, --host stringSlice        host list. Default is local sockets
      --httptest.serve string   if non-empty, httptest.NewServer serves on this address and blocks
      --log-caller              include caller function (default true)
      --log-format string       log format: logfmt|term|json (default "term")
      --log-stack               include caller stack
      --log-stdout              log to stdout


$ infrakit util event-repeater --source ~/.infrakit/plugins/event-time --sink tcp://localhost:1883
```

Now your app connected to event plugin and mqtt broker.
If you set `—-allowall`, your app subscribe ‘.’ Topic from event and publish all events to broker with original topic.
You can specify repeat topics with infrakit command like below.
Infrakit app: event-repeater serve REST API. 
At default, it listen with unix socket. 
If you want to use tcp socket instead of unix socket, set option like below.

```
infrakit util event-repeater --listen localhost:8080 --source ~/.infrakit/plugins/event-time --sink tcp://localhost:1883
```
## Manage event repeater

You can manipurate Infrakit application with `infrakit util application` command.

```
$ infrakit util application update -h

___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


Access application plugins

Usage:
  infrakit util application [command]

Available Commands:
  delete      Delete request to application.
  get         Get request to application.
  post        Post request to application.
  put         Put request to application.

Flags:
      --name string   Name of plugin
      --path string   URL path of resource e.g. /resources/resourceID/ (default "/")

Global Flags:
  -H, --host stringSlice        host list. Default is local sockets
      --httptest.serve string   if non-empty, httptest.NewServer serves on this address and blocks
      --log int                 log level (default 4)
      --log-caller              include caller function (default true)
      --log-format string       log format: logfmt|term|json (default "term")
      --log-stack               include caller stack
      --log-stdout              log to stdout

Use "infrakit util application [command] --help" for more information about a command.

```
As you see, you can send REST request with `get, post, put, delete` commands.
You do not have to consious about whether your application is listening on UNIX sockets or TCP ports.
Only specify your application name.
And with `--path` specify the target resource of the application.
For example, in event-repeater you should set `--path /events`
Except for `get` command, you can set json quary by `--value` option.
In the example below, you specify the event that is the target of repeate and the topic when publishing the event as mqtt.

```
infrakit util application --name app-event-repeater --path /events post --value '[{"sourcetopic":"timer/sec/1","sinktopic":"/time/1s"},{"sourcetopic":"timer/msec/500","sinktopic":"/time/500m"}]'
```

Ofcource, you can same operation with other tool e.g. `curl`.

```
TCP Port: curl -v -H "Accept: application/json" -H "Content-type: application/json" -X POST -d '[{"sourcetopic":"timer/sec/1","sinktopic":"/time/1s"},{"sourcetopic":"timer/msec/500","sinktopic":"/time/500m"}]'  http://localhost:8080/events
Unix Socket : curl -v -H "Accept: application/json" -H "Content-type: application/json" -X POST -d '[{"sourcetopic":"timer/sec/1","sinktopic":"/time/1s"},{"sourcetopic":"timer/msec/500","sinktopic":"/time/500m"}]'  --unix-socket /home/ubuntu/.infrakit/plugins/app-event-repeater.listen http:/events
```
Target events are described json style.
Then you can delete registerd event.

```
$ infrakit application update --name app-event-repeater --op 2 --resource event --value '[{"sourcetopic":"timer/sec/1”}]’
```
Repeated events are encoded with byte.
You can decode it like below.

```
any := types.AnyBytes(SUBSCRIVED_MESSAGE.Payload())
subevent := event.Event{}
err := any.Decode(&subevent)
```
