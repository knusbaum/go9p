# go9p
This is a golang implementation of the 9p2000 protocol.
Provided is foremost an API to implement servers serving the protocol, but also some more primitive constructs if you'd like to dink with the protocol outside the confines of what the API provides.

Examples are available in examples/ (duh)

Docs available here: http://godoc.org/github.com/knusbaum/go9p

These programs are meant to be used with plan9port's 9pfuse (or some equivalent)
https://github.com/9fans/plan9port

For example, you would mount the ramfs example with the following command:
```
9pfuse localhost:9999 /mnt/myramfs
```
Then you can copy files to/from the ramfs and do all the other stuff that you'd expect.

This is distributed under the MIT license

```
    Copyright (c) 2016 Kyle Nusbaum


    Permission is hereby granted, free of charge, to any person obtaining a copy
    of this software and associated documentation files (the "Software"), to deal
    in the Software without restriction, including without limitation the rights
    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
    copies of the Software, and to permit persons to whom the Software is furnished
    to do so, subject to the following conditions:

    The above copyright notice and this permission notice shall be included in all
    copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
    FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
    COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN
    AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
    WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

```