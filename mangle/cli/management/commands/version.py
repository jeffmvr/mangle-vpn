from mangle.cli.command import BaseCommand
from mangle import version


class Command(BaseCommand):
    help = "prints the application version number."

    def handle(self, *args, **options):
        self.print("Mangle VPN v{}".format(version.version()))
