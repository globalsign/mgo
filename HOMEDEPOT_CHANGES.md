Home Depot provides these changes for community use with the usual caveats - no warranty, etc.

This code contains the following changes - which will be pushed back to the parent repo (globalsign/mgo)

* 4.0 Transaction Support

Here's how to use them:

If you don't want to use 4.0 transaction support, do nothing.  It will not be enabled and nothing will happen.

If you need to use transaction support, do the following:

- Create a Transaction object

```
    err := m.Session.Start()
	if err != nil {
		panic(err.Error())
	}
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

Currently there are no tests, but we will be working on those before doing a pull request.  We're just making this available so the community can make use of them.

Caveats:

- You can create more than one transaction object, but only use one at a time.
- You can update the AutoCommit field in the transaction object before doing the first update operation.
- More docs to come.