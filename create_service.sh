#!/bin/sh
#Before running, ensure gsw_service is built via: go build cmd/gsw_service.go
servicefile=/etc/systemd/system/gsw.service
rm -f $servicefile
touch $servicefile
pwd
{ printf "[Unit]\n"; printf "Description=RIT Launch Ground Software Service\n\n"; } >> $servicefile

{ printf "[Service]\n"; printf "Type=Simple\n"; } >> $servicefile
if chmod 777 gsw_service;
then
    echo "gsw_service exists"
else
    echo "gsw_service not found, exiting"
    exit 1
fi
{ printf "Restart=on-failure\n"; }
{ printf "ExecStart=%s/gsw_service\n" "$(pwd)"; printf "WorkingDirectory=%s\n\n" "$(pwd)"; } >> $servicefile

{ printf "[Install]\n"; printf "WantedBy=multi-user.target\n"; } >> $servicefile

systemctl daemon-reload
systemctl enable gsw
systemctl start gsw
systemctl status gsw 
