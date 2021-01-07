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

using namespace std;


int main()
{
	int bExp = 0;




	unsigned char pKeyWord[512];

	for (int i = 0; i < 512; i++)
		pKeyWord[i] = rand() % 254;


	FILE *fp = NULL;

	fp = fopen("Keys.txt", "a+");

	fprintf(fp, "unsigned char pKeyWord[512] = {\n");

	for (int i = 0; i < 512; i++)
	{
		if (i % 20 == 0)
			fprintf(fp, "\n");

		fprintf(fp, "%d, ", pKeyWord[i]);
	}
	fprintf(fp, "\n};");
	fclose(fp);

	fp = fopen("Keys2.txt", "a+");

	fprintf(fp, "char pKeyWord[512];\n");

	for (int i = 0; i < 512; i++)
	{
		if (i % 6 == 0)
			fprintf(fp, "\n");

		fprintf(fp, "pKeyWord[%d] = %d; ", i, pKeyWord[i]);
	}

	fclose(fp);

	/*
	printf("Qual será a Exp base?\n");
	scanf("%d", &bExp);

	LoadMobsInDir("./npc/*", bExp);
	*/
	return 1;
}

void LoadMobsInDir(char *dir, int bExp)
{
	HANDLE handle;
	WIN32_FIND_DATA win32_find_data;

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
				sprintf(tmp, "./npc/*", fileName);
				LoadMobsInDir(tmp, bExp);
			}
			else
			{
				strcpy(tmp, dir);
				tmp[strlen(dir) - 1] = '\0';
				sprintf(tmp, "%s%s", tmp, fileName);

				errno = 0;

				FILE* fs;
				fopen_s(&fs, tmp, "rb");
				STRUCT_MOB MOB;
				memset(&MOB, 0, sizeof(STRUCT_MOB));

				if (fs != NULL)
				{
					int fsize = 0;

					fseek(fs, 0, SEEK_END);
					fsize = ftell(fs);
					fseek(fs, 0, SEEK_SET);
					fread(&MOB, 1, sizeof(STRUCT_MOB), fs);
					fclose(fs);

					fopen_s(&fs, tmp, "w");

					MOB.Exp = MOB.CurrentScore.Level < 35 ? bExp : MOB.CurrentScore.Level / 35 * bExp;

					fwrite(&MOB, sizeof(STRUCT_MOB), 1, fs);
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

	return;
}
