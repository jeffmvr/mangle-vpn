from rest_framework.pagination import PageNumberPagination


class ApiPagination(PageNumberPagination):
    page_size_query_param = "size"
