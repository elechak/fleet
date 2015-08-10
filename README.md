# Fleet v0.1
Go distributed computing with SSH

Fleet allows you to create and manage a "fleet" of remote interpreters.  

## Features
- All communication with the interpreters is encrypted using ssh.
- Use a multitude of users across all machines to help manage permissions
- Define machine groups; then easily create a pool of interpreters from those groups.  
- Resource manager that determines the best machines on which to start the interpreters.

## Languages Currently Supported
- python3
- lua
- perl
- bash
- node.js
- docker-bash
- docker-python3

## Examples
### Create a single remote bash interpreter and run hostname
```
i := fleet.NewInterp("bash", hostname, username, password, private_key)
i.Request("hostname")
i.Show()
```

### Explicitly use Write and Wait methods, show return timestamp
```
i := fleet.NewInterp("bash", hostname, username, password, private_key)
i.Write("hostname")
i.Wait(0.1)
i.Show()
fmt.Println(i.Time())
```

### Create a Group
```
group := fleet.NewGroup()

group.AddHost("host1").Login("user1", "pass1")
group.AddHost("host2").Login("user1", "pass1")
group.AddHost("host3").Login("user1", "pass1")
group.GetStatus()
group.Save("filename.grp")

```
### Use a Group to create a pool of interpreters
```
group := fleet.LoadGroup("filename.grp")
group.Show()

pool := group.Pool("bash", 5, 0.1) // language, max interpreters, est. memory consumption
fleet.Request(pool, "hostname")
fleet.Show(pool)
```



## Disclaimer
Fleet is still early in its development.  Things are still rapidly changing.

