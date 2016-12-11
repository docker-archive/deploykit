#!/bin/bash

echo "Notify AWS that server is ready"
cfn-signal --stack $STACK --resource $RESOURCE --region $REGION
