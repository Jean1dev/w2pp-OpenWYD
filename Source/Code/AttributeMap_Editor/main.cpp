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

#include <Windows.h>
#include <stdio.h>

unsigned char g_pAttribute[1024][1024];

int main()
{
	FILE *fp = fopen("AttributeMap.dat", "rb");

	if (fp == NULL)
	{
		MessageBoxA(NULL, "There is no file", "AttributeMap.dat", MB_OK);

		return FALSE;
	}

	fread(g_pAttribute, 1024, 1024, fp);

	fclose(fp);

	int xx = g_pAttribute[2100 / 4][2100 / 4];

	for (int x = 0; x < 1024; x++)
	{
		for (int y = 0; y < 1024; y++)
		{
			//if ((g_pAttribute[y][x] & 64) != 0 && (g_pAttribute[y][x] & 1) != 0)
			//	g_pAttribute[y][x] |= 64;

			if (g_pAttribute[y][x] < 64 && g_pAttribute[y][x] != 1)
				g_pAttribute[y][x] |= 64;
		}
	}
	
	fp = fopen("AttributeMap_New.dat", "wb");

	fwrite(g_pAttribute, 1024, 1024, fp);

	fclose(fp);

	//0: PvE sem PvP
	//1: Cidade
	//4: Não é permitido marcar Gema
	//64: PvP que perde XP
	system("pause");
	return 1;
}