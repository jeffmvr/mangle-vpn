from django.template.loader import render_to_string
from mangle.cli.command import BaseCommand
from mangle.common import config, iptables


class Command(BaseCommand):
    help = "performs web application tasks."

    def add_arguments(self, parser):
        parser.add_argument("action", help="the web action to perform")

    def handle(self, *args, **options):
        action = options["action"]

        if action == "post-start":
            web_post_start()
        elif action == "post-stop":
            web_post_stop()


######################################
# Actions
######################################

def web_post_start():
    """
    Creates the web application firewall rules.
    :return: None
    """
    web_post_stop()

    rules = _render_rules()

    for rule in _render_rules().split("\n"):
        iptables.run(rule)

    # save the firewall rules so they can be properly removed
    config.set("web_firewall_rules", rules)


def web_post_stop():
    """
    Deletes the web application firewall rules.
    :return: None
    """
    rules = config.get("web_firewall_rules", "")

    for rule in rules.split("\n"):
        iptables.run(rule.replace("-A", "-D"))

    config.delete("web_firewall_rules")


def _render_rules():
    """
    Renders and returns the web application firewall rules template.
    :return: str
    """
    return render_to_string(
        template_name="firewall/web.rules",
        context={
            "http_port": config.get("app_http_port", 80),
            "https_port": config.get("app_https_port", 443),
        },
    )
