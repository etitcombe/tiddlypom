# tiddlypom
A web application for hosting your own [TiddlyWiki](https://tiddlywiki.com/),
written in Go.

This is an attempt at implementing a server for [TiddlyWiki](https://tiddlywiki.com/).
The idea is you can run this application on your own server. It is heavily
inspired by https://github.com/rsc/tiddly but with data stored in SQLite instead
of Google App Engine.

## Why?
My stomach turns at the thought of installing NodeJS and npm on my VPS. I imagine
there are ways of packaging a JavaScript application into a single executable,
but that's not the default. Also, Go is fun.

I know, this app isn't as good as the one that comes with TiddlyWiki itself.
Plugins are more difficult to manage, there are various hacks around the system
tiddlers, who knows what other functionality is broken. But this is fun.

### Authentication
The assumption is that this is installed on your own server so that you can access
it from anywhere. With that in mind, the application is secured.

It's a bit unfriendly, you need to create two files in the same folder as the
web application executable: 1) .config and 2) users.gob.

.config should contain a JSON object with one property, "pepper":

    {
        "pepper": "[your value goes here]"
    }

That pepper value and the users.gob file can be generated with the included
cmd/admin application.