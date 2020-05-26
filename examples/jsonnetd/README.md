## jsonnetd

This is an HTTP server that loads a directory of Jsonnet files and
serves each one as a webhook.

Each hook should evaluate to a Jsonnet function:

```js
function(request) {
  // response body
}
```

The body of the POST request is itself interpreted as Jsonnet
and given to the hook as a top-level `request` argument.

The entire result of evaluating the Jsonnet function is returned as
the webhook response body, unless the function returns an error.
