import os
import sys
import textwrap

from colorama import Fore, Style
from django.core import management


class BaseCommand(management.BaseCommand):
    text_width = 60

    def header(self, title):
        """
        Prints the given title header.
        :return: None
        """
        self.newline()
        print(Fore.CYAN + Style.BRIGHT + title + Style.RESET_ALL)

    def print(self, message, *args):
        """
        Prints the given message.
        :return: None
        """
        self._output(Fore.WHITE, message, *args)

    def bold(self, message, *args):
        """
        Prints the given bold message.
        :return: None
        """
        message = Style.BRIGHT + message + Style.RESET_ALL
        self.print(message, *args)

    def error(self, message, *args):
        """
        Prints the given error message.
        :return: None
        """
        self._output(Fore.RED, message, *args)

    def info(self, message, *args):
        """
        Prints the given info message.
        :return: None
        """
        self._output(Fore.CYAN, message, *args)

    def ok(self, message, *args):
        """
        Prints the given success message.
        :return: None
        """
        self._output(Fore.GREEN, message, *args)

    def warn(self, message, *args):
        """
        Prints the given warning message.
        :return: None
        """
        self._output(Fore.YELLOW, message, *args)

    def newline(self, count=1):
        """
        Prints the given number of newlines.
        :return: None
        """
        [print("\n") for _ in range(count)]

    def title(self):
        """
        Prints the application title.
        :return: None
        """
        os.system("clear")
        print(Fore.MAGENTA + title_ascii + Fore.RESET)

    def exit(self, message=None, *args):
        """
        Exits the application with the given error message.
        :return: None
        """
        self.error(message, *args)
        self.newline()
        sys.exit(1)

    def _output(self, color, message, *args):
        """
        Prints the given message and wraps based on the configured text width.
        :return: None
        """
        for line in textwrap.wrap(message.format(*args), self.text_width):
            print(color + line + Fore.RESET)


title_ascii = """
                             _                          
 _ __ ___   __ _ _ __   __ _| | ___  __   ___ __  _ __  
| '_ ` _ \ / _` | '_ \ / _` | |/ _ \\ \ \ / / '_ \| '_ \ 
| | | | | | (_| | | | | (_| | |  __/  \ V /| |_) | | | |
|_| |_| |_|\__,_|_| |_|\__, |_|\___|   \_/ | .__/|_| |_|
                       |___/               |_|          
"""
