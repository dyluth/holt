#!/bin/sh
# Example agent run script for M2.2 testing
# This agent doesn't actually execute work - it just demonstrates the cub architecture
# In M2.3, real agents will execute tools based on granted claims

# For M2.2, we just sleep forever to keep the container running
# The cub will handle claim watching, bidding, and grant reception
echo "Example agent started (sleep mode for M2.2 testing)"
sleep infinity
