from mangle.common import config


def config_middleware(get_response):
    """
    Middleware that reloads the application config on each request.
    :returns: Response
    """
    def middleware(request):
        config.reload()
        return get_response(request)
    return middleware
