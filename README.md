# go9p
This is a golang implementation of the 9p2000 protocol.
Provided are foremost an API to implement servers serving the protocol, but also some more primitive constructs if you'd like to dink with the protocol outside the confines of what the API provides.

The parser and other primitive stuff can be found in fcall/. fcall/fcall.go is a good place to start.

(Currently the API mentioned above isn't implemented. I plan on doing that this week, along with writing a better readme and hopefully doing some code documentation)


Check out https://github.com/knusbaum/9ptest for a sample application serving a ram-backed 9P filesystem based on this library.
