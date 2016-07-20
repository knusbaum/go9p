# go9p
This is a golang implementation of the 9p2000 protocol.
Provided is foremost an API to implement servers serving the protocol, but also some more primitive constructs if you'd like to dink with the protocol outside the confines of what the API provides.

The parser and other primitive stuff can be found in fcall/. fcall/fcall.go is a good place to start.

(Currently the API mentioned above isn't implemented. I plan on doing that this week, along with writing a better readme and hopefully doing some code documentation)


Check out https://github.com/knusbaum/9ptest for a sample application serving a ram-backed 9P filesystem based on this library.

This is distributed under the MIT license

```
    Copyright (c) 2016 Kyle Nusbaum


    Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

    The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

```