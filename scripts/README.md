# Create Service Script
This script must be run from the /scripts directory.
gsw_service must be built prior to the script being run (and it must exist for the service to work).

Once the script has been run, start the service with:
sudo systemctl start gsw

Stop the service with:
sudo systemctl stop gsw

If you want the sevrice to run on startup:
sudo systemctl enable gsw