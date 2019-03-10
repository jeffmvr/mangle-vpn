import subprocess


def run(*args, **kwargs):
    """
    Runs a BASH command with the given arguments and returns whether the
    command process exited successfully.
    :return: bool
    """
    code, out, err = run_output(*args, **kwargs)
    return code == 0


def run_output(*args, **kwargs):
    """
    Runs a BASH command with the given arguments and returns a tuple containing
    the command process exit code, the standard output, and standard error.
    :return: Tuple[int,str,str]
    """
    command = " ".join([str(arg) for arg in args])

    proc = subprocess.Popen(
        command,
        executable="/bin/bash",
        shell=True,
        stderr=subprocess.PIPE,
        stdout=subprocess.PIPE,
        **kwargs,
    )

    # this call will block until the process finishes
    stdout, stderr = proc.communicate()

    return (proc.returncode,
            stdout.decode("utf-8").strip(),
            stderr.decode("utf-8").strip())
