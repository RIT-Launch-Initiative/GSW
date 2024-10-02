#!/bin/sh
# Before running, ensure gsw_service is built via: go build cmd/gsw_service.go
# Must be run from /scripts directory
servicefile=/etc/systemd/system/gsw.service
rm $servicefile
touch $servicefile
chmod 664 $servicefile

wd=$(pwd)
wd=$(echo $wd | rev | cut -d'/' -f2- | rev)

{ printf "[Unit]\n"; printf "Description=RIT Launch Ground Software Service\n\n"; } >> $servicefile

{ printf "[Service]\n";} >> $servicefile
if chmod 777 ../gsw_service;
then
    echo "gsw_service exists"
else
    echo "gsw_service not found, exiting"
    exit 1
fi

{ printf "WorkingDirectory=%s\n" "$wd"; printf "ExecStart=%s/gsw_service\n" "$wd"; } >> $servicefile
{ printf "Type=simple\n"; printf "UMask=0002\n"; printf "User=%s\n" "$(logname)"; printf "Restart=on-failure\n\n"; } >> $servicefile

{ printf "[Install]\n"; printf "WantedBy=multi-user.target\n"; } >> $servicefile

systemctl daemon-reload
systemctl disable gsw
