from .utils import bash


def get_chains(table):
    """
    Returns a list containing all of the chains from the given table.
    :return: List[str]
    """
    code, out, err = run_output("-t", table, "-S")

    chains = []
    for line in out.split("\n"):
        if line.startswith("-N"):
            chains.append(line.split("-N")[1])

    return chains


def get_rules(table, chain=""):
    """
    Returns all of the rules from the given chain. If the chain is not given,
    then returns all of the rules from the given table.
    :return: List[str]
    """
    code, out, err = run_output("-t", table, "-S", chain)
    return parse_rules(out)


def flush(table, chain=""):
    """
    Flushes all of the rules from the given chain. If the chain is not given,
    then flushes all of the rules from the given table.
    :return: bool
    """
    return run("-t", table, "-F", chain)


def get_chain_targets(table, chain):
    """
    Returns a list containing all of the rules that target the given chain.
    :return: List[str]
    """
    code, out, err = run_output("-t", table, "-S")

    rules = []
    for line in out.split("\n"):
        if "-j {}".format(chain) in line:
            rules.append(line)

    return rules


def create_chain(table, chain):
    """
    Creates the given chain.
    :return: bool
    """
    return run("-t", table, "-N", chain)


def delete_chain(table, chain):
    """
    Deletes the given chain (flushes the chain first).
    :return: bool
    """
    flush(table, chain)
    return run("-t", table, "-X", chain)


def rename_chain(table, chain, name):
    """
    Renames the given chain.
    :return: bool
    """
    return run("-t", table, "-E", chain, name)


def chain_exists(table, chain):
    """
    Returns whether the given chain exists.
    :return: bool
    """
    return chain in get_chains(table)


def append_rule(table, chain, *rule):
    """
    Appends the given rule to the end of a chain.
    :return: bool
    """
    return run("-t", table, "-A", chain, *rule)


def append_unique_rule(table, chain, *rule):
    """
    Appends the given rule to the end of a chain only if the rule doesn't
    already exist. If the rule exists, this does nothing.
    :return: bool
    """
    if not rule_exists(table, chain, *rule):
        return append_rule(table, chain, *rule)
    return True


def insert_rule(table, chain, position, *rule):
    """
    Inserts the given rule into a specific position in a chain.
    :return: bool
    """
    return run("-t", table, "-I", position, chain, *rule)


def insert_unique_rule(table, chain, position, *rule):
    """
    Inserts the given rule into a specific position in a chain only if the rule
    doesn't already exist. If the rule exists, this does nothing.
    :return: bool
    """
    if not rule_exists(table, chain, *rule):
        return insert_rule(table, chain, position, *rule)
    return True


def delete_rule(table, chain, *rule):
    """
    Deletes the first instance of the given rule from a chain.
    :return: bool
    """
    return run("-t", table, "-D", chain, *rule)


def clear_rule(table, chain, *rule):
    """
    Deletes all instances of the given rule from the given chain.
    :return: bool
    """
    while rule_exists(table, chain, *rule):
        if not delete_rule(table, chain, rule):
            return False
    return True


def rule_exists(table, chain, *rule):
    """
    Returns whether the given rule exists in the given chain.
    :return: bool
    """
    return run("-t", table, "-C", chain, *rule)


def parse_rules(output):
    """
    Returns a list containing all of the rules parsed from the given iptables
    command output.
    :return: List[str]
    """
    rules = []

    for line in output.split("\n"):
        if line.startswith("-A"):
            rules.append(line)

    return rules


def run(*args):
    """
    Returns whether the given iptables command executed successfully.
    :return: bool
    """
    code, out, err = run_output(*args)
    return code == 0


def run_output(*args):
    """
    Returns the output returned from the given iptables command.
    :return: Tuple[int,str,str]
    """
    if len(args) > 0 and args[0] == "":
        return 0, None, None

    # only execute the command if args were given
    return bash.run_output("iptables", "--wait", *args)
