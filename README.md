# Conntrack Exporter

The prometheus node exporter exports conntrack metrics off the /proc pseudo file
system. The conntrack module developers consider that surface as deprecated and
provide a CLI tool `conntrack` that shows more interesting metrics. 

This exporter exports values from running `conntrack -S`, e.g.:

```
cpu=0   	found=0 invalid=0 ignore=2970 insert=0 insert_failed=0 drop=0 early_drop=0 error=0 search_restart=0 
cpu=1   	found=0 invalid=0 ignore=2568 insert=0 insert_failed=0 drop=0 early_drop=0 error=0 search_restart=0 
cpu=2   	found=0 invalid=0 ignore=2160 insert=0 insert_failed=0 drop=0 early_drop=0 error=0 search_restart=517 
cpu=3   	found=0 invalid=0 ignore=2989 insert=0 insert_failed=0 drop=0 early_drop=0 error=0 search_restart=188 
```

```
# HELP conntrack_drop Total of conntrack drop
# TYPE conntrack_drop counter
conntrack_drop{cpu="0"} 0
conntrack_drop{cpu="1"} 0
conntrack_drop{cpu="2"} 0
conntrack_drop{cpu="3"} 0
# HELP conntrack_early_drop Total of conntrack early_drop
# TYPE conntrack_early_drop counter
conntrack_early_drop{cpu="0"} 0
conntrack_early_drop{cpu="1"} 0
conntrack_early_drop{cpu="2"} 0
conntrack_early_drop{cpu="3"} 0
# HELP conntrack_error Total of conntrack error
# TYPE conntrack_error counter
conntrack_error{cpu="0"} 0
conntrack_error{cpu="1"} 0
conntrack_error{cpu="2"} 0
conntrack_error{cpu="3"} 0
# HELP conntrack_found Total of conntrack found
# TYPE conntrack_found counter
conntrack_found{cpu="0"} 0
conntrack_found{cpu="1"} 0
conntrack_found{cpu="2"} 0
conntrack_found{cpu="3"} 0
# HELP conntrack_ignore Total of conntrack ignore
# TYPE conntrack_ignore counter
conntrack_ignore{cpu="0"} 2970
conntrack_ignore{cpu="1"} 2568
conntrack_ignore{cpu="2"} 2160
conntrack_ignore{cpu="3"} 2989
# HELP conntrack_insert Total of conntrack insert
# TYPE conntrack_insert counter
conntrack_insert{cpu="0"} 0
conntrack_insert{cpu="1"} 0
conntrack_insert{cpu="2"} 0
conntrack_insert{cpu="3"} 0
# HELP conntrack_insert_failed Total of conntrack insert_failed
# TYPE conntrack_insert_failed counter
conntrack_insert_failed{cpu="0"} 0
conntrack_insert_failed{cpu="1"} 0
conntrack_insert_failed{cpu="2"} 0
conntrack_insert_failed{cpu="3"} 0
# HELP conntrack_invalid Total of conntrack invalid
# TYPE conntrack_invalid counter
conntrack_invalid{cpu="0"} 0
conntrack_invalid{cpu="1"} 0
conntrack_invalid{cpu="2"} 0
conntrack_invalid{cpu="3"} 0
# HELP conntrack_search_restart Total of conntrack search_restart
# TYPE conntrack_search_restart counter
conntrack_search_restart{cpu="0"} 0
conntrack_search_restart{cpu="1"} 0
conntrack_search_restart{cpu="2"} 517
conntrack_search_restart{cpu="3"} 188
```