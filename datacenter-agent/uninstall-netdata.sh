#!/usr/bin/env bash

sudo systemctl stop netdata
sudo systemctl disable netdata
sudo rm  -rf /opt/netdata/
sudo groupdel netdata
sudo userdel netdata
sudo rm -rf /etc/logrotate.d/netdata
sudo rm -rf /etc/systemd/system/netdata.service
