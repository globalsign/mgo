
Home Depot provides these changes for community use with the usual
caveats - no warranty, etc.

This code contains the following changes - which will be pushed back
to the parent repo (globalsign/mgo)

* 4.0 Transaction Support

## How To Use Transaction Support

If you don't want to use 4.0 transaction support, do nothing.
It will not be enabled and nothing will happen.

If you need to use transaction support, do the following:

- Create a Transaction object

```
// This populates the Session object with a Session ID.
// This is required for transactions, but not in other cases.
err := m.Session.Start()
if err != nil {
    panic(err.Error())
}
// Create a new transaction object
tr := mgo.NewTransaction(m.Session)
m.Transaction = &tr
```

- Use the provided update functions

```
c.UpsertTransaction(tr, ...)
c.InsertTransaction(tr, ...)
c.RemoveTransaction(tr, ...)
c.UpdateTransaction(tr, ...)
```

- Commit or abort the transactions when you are finished.

```
tr.Commit()
tr.Abort()
```

## Replication

In order to adjust the tests to use replication (because the default
server install doesn't use replication), we made adjustments to allow
the dbtest server to set up as a single-server replica set.  To enable,
instead of

```
var Server DBServer
Server.Session()
```

use

```
var Server DBServer
Server.SessionRepl(true)
```

You can also use

```
Server.SessionRepl(false)
```

to duplicate the behavior of
```
Server.Session()
```


Caveats:

- You can create more than one transaction object, but only use one at a time.
- More docs to come.