# A2A-GO

Please read the README.md for general information.

Always refer to the specs directory, where you will find the specifications for the protocols we are using.

We are building a distributed agent framework, according to the agent-2-agent protocol, combined with the MCP protocol.

The idea is that each agent, each tool, and each service, runs in its own container, and communicates with the other agents, tools, and services, using the protocols we are using.

It is very important that the "public facing" api (what developers will use to experiment with agents) is very simple, and easy to use.

Please continue development, making sure to follow the specs, and keep your code clean, well organized, and easy to understand.

Always use Godoc comments above any methods and types, using the /**/ format.

Use Goconvey for tests and always have one test function per code function, so the tests mirror the structure of the code.

Please follow the already established code style.

There is also a TUI component, which is used for testing and interacting with the system, so do not forget to update the TUI when you make changes to the code.
