/*
*   Copyright (C) {2015}  {Victor Klafke, Charles TheHouse}
*
*   This program is free software: you can redistribute it and/or modify
*   it under the terms of the GNU General Public License as published by
*   the Free Software Foundation, either version 3 of the License, or
*   (at your option) any later version.
*
*   This program is distributed in the hope that it will be useful,
*   but WITHOUT ANY WARRANTY; without even the implied warranty of
*   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
*   GNU General Public License for more details.
*
*   You should have received a copy of the GNU General Public License
*   along with this program.  If not, see [http://www.gnu.org/licenses/].
*
*   Contact at: victor.klafke@ecomp.ufsm.br
*/
#include "main.h"

FILE *fp = NULL;

int readAccountsInDir(char *dir, char *pass)
{
	HANDLE handle;
	WIN32_FIND_DATA win32_find_data;

	int entryCount = 0;

	char tmp[1024];

	handle = FindFirstFile(dir, &win32_find_data);

	if (handle == INVALID_HANDLE_VALUE)
	{
		// TODO: Error: can't read initial file
	}
	else
	{
		while (FindNextFile(handle, &win32_find_data))
		{
			char *fileName = win32_find_data.cFileName;

			if (!strncmp(fileName, ".", 1))
				continue;

			if (win32_find_data.dwFileAttributes == FILE_ATTRIBUTE_DIRECTORY)
			{
				sprintf(tmp, "./account/%s/*", fileName);
				entryCount += readAccountsInDir(tmp, pass);
			}
			else
			{
				strcpy(tmp, dir);
				tmp[strlen(dir) - 1] = '\0';
				sprintf(tmp, "%s%s", tmp, fileName);

				errno = 0;

				FILE* fs;
				fopen_s(&fs, tmp, "rb");

				STRUCT_ACCOUNTFILE accBuffer;

				if (fs != NULL)
				{
					int fsize = 0;

					fseek(fs, 0, SEEK_END);
					fsize = ftell(fs);
					fseek(fs, 0, SEEK_SET);

					if ((fsize >= 7500 && fsize <= 8900) || fsize == sizeof(STRUCT_ACCOUNTFILE))
					{
						//STRUCT_ACCOUNTFILE accBuffer;
						memset(&accBuffer, 0, sizeof(STRUCT_ACCOUNTFILE));

						fread(&accBuffer, sizeof(STRUCT_ACCOUNTFILE), 1, fs);

						if (strncmp(accBuffer.Info.AccountPass, pass, strlen(pass)) == 0)
							fprintf(fp, "%s\n", accBuffer.Info.AccountName);
						
					}
					else
					{
						// TODO: file don't have the right size
					}

					fclose(fs);
				}
				else
				{
					// TODO: Cant open file to read
				}
			}
		}
	}

	FindClose(handle);

	return entryCount;
}

int main()
{
	char pass[256];

	printf("Qual a senha para procurar jovem?\n");

	scanf("%s", pass);

	fopen_s(&fp, "RAKER.txt", "a+");

	readAccountsInDir("./account/*", pass);

	fclose(fp);
	printf("Terminado.\n");
	system("pause");
	return 1;
}