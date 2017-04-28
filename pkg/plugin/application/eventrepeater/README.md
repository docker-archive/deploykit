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
$ ./build/infrakit-application-repeater -h
Event Repeater Application plugin

Usage:
  ./build/infrakit-application-repeater [flags]

Flags:
      --allowall              Allow all event from source and repeat the event to sink as same topic name. default: false
      --log int               Logging level. 0 is least verbose. Max is 5 (default 4)
      --name string           Application name to advertise for discovery (default "app-event-repeater")
      --sink string           Event sink address. default: localhost:1883 (default "localhost:1883")
      --sinkprotocol string   Event sink protocol. Now only mqtt and stderr is implemented. (default "mqtt")
      --source string         Event sourve address. (default "event-plugin")

$ ./build/infrakit-application-repeater --source ~/.infrakit/plugins/event-time --sink tcp://localhost:1883
```

Now your app connected to event plugin and mqtt broker.
If you set `—-allowall`, your app subscribe ‘.’ Topic from event and publish all events to broker with original topic.
You can specify repeat topics with infrakit command like below.

```
$ ./build/infrakit application update -h


___  ________   ________ ________  ________  ___  __    ___  _________
|\  \|\   ___  \|\  _____\\   __  \|\   __  \|\  \|\  \ |\  \|\___   ___\
\ \  \ \  \\ \  \ \  \__/\ \  \|\  \ \  \|\  \ \  \/  /|\ \  \|___ \  \_|
 \ \  \ \  \\ \  \ \   __\\ \   _  _\ \   __  \ \   ___  \ \  \   \ \  \
  \ \  \ \  \\ \  \ \  \_| \ \  \\  \\ \  \ \  \ \  \\ \  \ \  \   \ \  \
   \ \__\ \__\\ \__\ \__\   \ \__\\ _\\ \__\ \__\ \__\\ \__\ \__\   \ \__\
    \|__|\|__| \|__|\|__|    \|__|\|__|\|__|\|__|\|__| \|__|\|__|    \|__|


Update application's resouce

Usage:
  ./build/infrakit application update [flags]

Flags:
      --op int            update operation 1: Add, 2: Delete, 3: Update, 4: Read(default) (default 3)
      --resource string   target resource
      --value string      update value

Global Flags:
  -H, --host stringSlice        host list. Default is local sockets
      --httptest.serve string   if non-empty, httptest.NewServer serves on this address and blocks
      --log int                 log level (default 4)
      --log-caller              include caller function (default true)
      --log-format string       log format: logfmt|term|json (default "term")
      --log-stack               include caller stack
      --log-stdout              log to stdout
      --name string             Name of plugin
$ ./build/infrakit application update --name app-event-repeater --op 1 --resource event --value '[{"sourcetopic":"timer/sec/1","sinktopic":"/time/1s"},{"sourcetopic":"timer/msec/500","sinktopic":"/time/500m"}]'
```

Target events are described json style.
Then you can delete registerd event.

```
./build/infrakit application update --name app-event-repeater --op 2 --resource event --value '[{"sourcetopic":"timer/sec/1”}]’
```
Repeated events are encoded with byte.
You can decode it like below.

```
any := types.AnyBytes(SUBSCRIVED_MESSAGE.Payload())
subevent := event.Event{}
err := any.Decode(&subevent)
```
